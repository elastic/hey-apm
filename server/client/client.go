package client

import (
	"bufio"
	"fmt"
	stdio "io"
	"os"
	"os/exec"
	"path/filepath"
	s "strings"
	"sync"
	"syscall"
	"time"

	"net"

	"context"

	"net/url"
	"path"

	"strconv"

	"runtime"

	"github.com/elastic/hey-apm/server/api"
	"github.com/elastic/hey-apm/server/api/io"
	"github.com/elastic/hey-apm/server/docker"
	"github.com/elastic/hey-apm/server/strcoll"
	"github.com/olivere/elastic"
	"github.com/pkg/errors"
)

type RWC interface {
	stdio.Writer
	stdio.ReadCloser
}

type Connection struct {
	RWC
	// used for the server to push commands that modify the evaluation environment state,
	// and the client to pull them
	EvalCh chan []string
	// used to inspect the contents of EvalCh without triggering a race condition
	LockCh *sync.WaitGroup
	// signal to abort running load tests
	// can be triggered by the user, connection termination, or apm-server process termination
	CancelSig *sync.Cond
}

// wraps a tcp connection with a buffered channel and sync mechanisms
// the channel holds commands that the server sends to the client for their evaluation
func WrapConnection(c net.Conn) Connection {
	return Connection{
		c,
		make(chan []string, 1000),
		&sync.WaitGroup{},
		&sync.Cond{L: &sync.Mutex{}},
	}
}

func (c Connection) waitForCancel() {
	c.CancelSig.L.Lock()
	defer c.CancelSig.L.Unlock()
	c.CancelSig.Wait()
}

type evalEnvironment struct {
	*es
	*apm
	nameDefs map[string][]string
}

// loads from disk user-defined names and last working elasticsearch/apm configuration, if any
// the evaluation environment models everything that impacts the evaluation of user's commands,
// other than the commands themselves
// assumes r/w access to home directory, posix, git, go and docker installed and globally available
func NewEvalEnvironment(usr string) *evalEnvironment {
	_, es := elasticSearchUse(usr, "last")
	_, apm := apmUse(usr, "last")
	env := evalEnvironment{
		es:       es,
		apm:      apm,
		nameDefs: io.LoadDefs(usr),
	}
	return &env
}

type es struct {
	*elastic.Client
	url      string
	username string
	password string
	// index name where to save test workload reports
	reportIndex string
	// not nil when it can't connect to a node
	useErr error
}

type apm struct {
	// either directory of the apm-server repo or the string "docker"
	dir      string
	branch   string
	revision string
	revDate  string
	// to show in status command
	prettyRev string
	// if there are unstaged changes, tests can run but can't be saved because code won't be available
	unstaged bool
	// either the apm-server process itself when running it locally or a docker client, not the containerized process
	cmd *exec.Cmd
	// apm-server/docker log
	log []string
	mu  sync.RWMutex // guards over log
	// when dir is wrong
	useErr error
}

var fileWriter = io.DiskWriter{}

// pulls commands from conn's channel and evaluates them sequentially (ie: blocking its goroutine)
// if a command is valid, its evaluation modifies the evaluation environment state
func (env *evalEnvironment) EvalAndUpdate(usr string, conn Connection) {
	for cmd := range conn.EvalCh {
		// out and err is what gets written to the TCP connection
		var err error
		out := fmt.Sprintf("%s nothing done for %s", io.Red, s.Join(cmd, " "))

		fn := strcoll.Nth(0, cmd)
		arg1 := strcoll.Nth(1, cmd)
		args1 := strcoll.Rest(1, cmd)
		args2 := strcoll.Rest(2, cmd)
		switch {
		case fn == "define":
			//todo add as well reports keywords
			reservedWords := []string{
				"apm", "cancel", "collate", "define", "dump", "elasticsearch", "help", "quit", "status", "test", "verify",
			}
			out, env.nameDefs = api.Define(usr, fileWriter, reservedWords, args1, env.Names())
		case fn == "elasticsearch" && arg1 == "use":
			out, env.es = elasticSearchUse(usr, args2...)
			err = env.es.useErr
		case fn == "apm" && arg1 == "use":
			out, env.apm = apmUse(usr, strcoll.Nth(2, cmd))
			err = env.apm.useErr
		case fn == "apm" && arg1 == "switch":
			if env.apm.useErr != nil {
				// `apm switch` depends on `apm use` having succeeded
				err = env.apm.useErr
			} else {
				args, opts := io.ParseCmdOptions(args2)
				branch := strcoll.Nth(0, args)
				rev := strcoll.Nth(1, args)
				// apmSwitch writes to the connection right away, out and err must have zero value to not duplicate the message
				out, env.apm = apmSwitch(conn, env.apm.Dir(), branch, rev, opts)
			}

		case fn == "test":
			// ensure preconditions are met
			if env.apm.useErr != nil {
				err = env.apm.useErr
				break
			}
			if env.apm.branch == "" || env.apm.revision == "" {
				err = errors.New("unknown branch/revision")
				break
			}

			// parse optional arguments
			var limit string
			args1, limit = io.ParseCmdOption(args1, "--mem", "4g", true)
			if !docker.IsDockerized(env.apm) {
				limit = "-1"
			}
			flags := apmFlags(*env.es, env.apm.Url(), strcoll.Rest(5, args1))

			// deletes apm-* indices and starts apm-server process
			err = reset(env.es)
			if err == nil {
				err, env.apm = apmStart(conn, *env.apm, conn.CancelSig.Broadcast, flags, limit)
			}
			if err != nil {
				break
			}

			// load test and teardown
			var report api.TestReport
			out, report = api.LoadTest(conn, env, conn.waitForCancel, args1...)

			var mem int64
			if env.apm.IsRunning() {
				if docker.IsDockerized(env.apm) {
					mem, err = stopDocker(conn)
				} else {
					// first must kill the process, then gets its memory usage with Go API
					env.apm.cmd.Process.Kill()
					mem = maxRssUsed(env.apm.cmd)
				}
			}

			// validate and save results
			report = fillMissing(report, usr, env.apm.revision, env.apm.revDate, mem, docker.ToBytes(limit), flags)
			out2, ok := report.Validate(env.apm.unstaged)
			out = out + out2
			if ok {
				out = out + indexReport(env.es.Client, env.es.reportIndex, report)
			}
		}

		io.ReplyEitherNL(conn, err, out)
		io.Prompt(conn)
		// blocks if cancel <cmdId> or status command is in progress
		conn.LockCh.Wait()
	}
}

// performs git fetch, git checkout of given branch / revision, make and make update
// options are unused with a dockerized apm-server
// first returned argument is always "" because the output is printed as soon is produced
// (ie. `w` is the actual tcp connection)
func apmSwitch(w stdio.Writer, apmDir, branch, revision string, opts []string) (string, *apm) {
	apm := newApm(apmDir)
	verbose := strcoll.ContainsAny(opts, "v", "verbose")
	dir := apm.Dir()
	if docker.IsDockerized(apm) {
		dir = docker.Dir()
	}
	var sh = io.Shell(w, dir, verbose)

	tokens := s.Split(branch, "/")

	// true is just a harmless command
	fetchCmd := "true"
	if strcoll.ContainsAny(opts, "f", "fetch") || docker.IsDockerized(apm) {
		if len(tokens) == 2 {
			fetchCmd = "git fetch " + tokens[0]
		} else {
			fetchCmd = "git fetch origin"
		}
	}

	checkoutCmd := "true"
	if strcoll.ContainsAny(opts, "c", "checkout") || docker.IsDockerized(apm) {
		if revision != "" {
			checkoutCmd = "git checkout " + revision
		} else {
			checkoutCmd = "git checkout " + branch
		}
	}

	makeUpdateCmd := "true"
	if strcoll.ContainsAny(opts, "u", "make-update") || docker.IsDockerized(apm) {
		makeUpdateCmd = "make update"
	}

	makeCmd := "true"
	if strcoll.ContainsAny(opts, "m", "make") || docker.IsDockerized(apm) {
		makeCmd = "make"
	}

	// eg elastic/master => master
	if len(tokens) == 2 {
		branch = tokens[1]
	}

	if docker.IsDockerized(apm) {
		cacheKey := docker.Image(branch, revision)
		if revision == "" {
			// do not cache HEAD because it will point to different things over time
			cacheKey = strconv.FormatInt(time.Now().UnixNano(), 10)
		}
		sh("docker", "build", "--label", docker.Label(), "-t", docker.Image(branch, revision),
			"--build-arg", "fetch_cmd="+fetchCmd,
			"--build-arg", "checkout_cmd="+checkoutCmd,
			"--build-arg", "make_update_cmd="+makeUpdateCmd,
			"--build-arg", "make_cmd="+makeCmd,
			"--build-arg", "nocache="+cacheKey,
			".")
	} else {
		sh(s.Split(fetchCmd, " ")...)
		sh(s.Split(checkoutCmd, " ")...)
		sh(s.Split(makeUpdateCmd, " ")...)
		sh(s.Split(makeCmd, " ")...)
	}

	// save relevant git status data if no errors occurred
	// todo: use only plumbing commands
	if _, err := sh(""); err == nil {
		if revision == "" {
			if docker.IsDockerized(apm) {
				revision, err = sh("docker", "run", "-i", "--rm", docker.Image(branch, revision), "git", "rev-parse", "HEAD")
				_, err = sh("docker", "tag", docker.Image(branch, ""), docker.Image(branch, revision))
			} else {
				revision, err = sh("git", "rev-parse", "HEAD")
			}
		}
		if err != nil || branch == revision {
			// apm.branch == revision is used to ensure that user don't simply pass a revision as a first argument
			// we always need both the branch and the revision
			return "", apm
		}
		apm.branch = branch
		apm.revision = revision

		if docker.IsDockerized(apm) {
			revDate, _ := sh("docker", "run", "-i", "--rm", docker.Image(branch, revision), "git", "show", "-s", "--format=%cd", "--date=rfc", revision)
			if t, _ := time.Parse(io.GITRFC, revDate); !t.IsZero() {
				apm.revDate = revDate
			}
			apm.unstaged = false
			apm.prettyRev, _ = sh("docker", "run", "-i", "--rm", docker.Image(branch, revision), "git", "log", "-1", "--oneline")

		} else {
			revDate, _ := sh("git", "show", "-s", "--format=%cd", "--date=rfc", revision)
			if t, _ := time.Parse(io.GITRFC, string(revDate)); !t.IsZero() {
				apm.revDate = revDate
			}
			if ok, err := sh("git", "diff", "HEAD"); err == nil && ok == "" {
				apm.unstaged = false
			}
			apm.prettyRev, _ = sh("git", "log", "-1", "--oneline")
		}
	}

	return "", apm
}

// starts apm with the given arguments
// injects output.elasticsearch and apm-server.host configuration from the current state
func apmStart(w stdio.Writer, apm apm, cancel func(), flags []string, limit string) (error, *apm) {
	newApm := newApm(apm.Dir())
	newApm.branch = apm.branch
	newApm.revision = apm.revision
	newApm.revDate = apm.revDate
	newApm.unstaged = apm.unstaged
	var cmd *exec.Cmd
	if docker.IsDockerized(newApm) {
		args := []string{"run", "--rm", "-i",
			"-p", "8200:8200",
			"--name", docker.Container(),
			"--memory=" + limit,
			// disallow swapping
			"--memory-swap=" + limit,
			docker.Image(newApm.branch, newApm.revision),
			"./apm-server"}
		args = append(args, flags...)
		cmd = exec.Command("docker", args...)
		cmd.Dir = docker.Dir()
	} else {
		cmd = exec.Command("./apm-server", flags...)
		cmd.Dir = apm.Dir()
	}

	io.ReplyNL(w, io.Cyan)
	io.ReplyWithDots(w, cmd.Args...)

	// apm-server writes to stderr by default, this consumes it as soon is produced
	stderr, err := cmd.StderrPipe()
	if err == nil {
		err = cmd.Start()
	}
	newApm.cmd = cmd

	scanner := bufio.NewScanner(stderr)
	go func() {
		// assumes default logging configuration
		var log []string
		for scanner.Scan() {
			if len(log) >= 1000 {
				// rotate log
				log = log[:+copy(log[:], append(log[1:], scanner.Text()))]
			} else {
				log = append(log, scanner.Text())
			}
			newApm.mu.Lock()
			newApm.log = log
			newApm.mu.Unlock()
		}
	}()
	go func() {
		time.Sleep(time.Millisecond * 500)
		err := cmd.Wait()
		// in case eg. apm server is killed externally (wont work eg. with docker stop)
		cancel()
		if err != nil && !s.Contains(err.Error(), "signal: killed") {
			io.ReplyNL(w, api.Tail(newApm.Log(), 5, ""))
			io.Prompt(w)
		}
	}()
	return err, newApm
}

// informs the apm-server directory, currently limited to localhost
// defaults to the expected Go location (make update might fail in a non default location)
// "last" loads from disk the last working directory
// "local" is the short for the expected location (GOPATH/src/...)
// "docker" will cause `apmSwitch` to build an image and `test` to run it
// writes to disk
func apmUse(usr, dir string) (string, *apm) {
	w := io.NewBufferWriter()
	if dir == "last" {
		dir = strcoll.Nth(0, io.LoadApmcfg(usr))
	}
	if dir == "local" {
		dir = filepath.Join(os.Getenv("GOPATH"), "/src/github.com/elastic/apm-server")
	}
	var err error
	if dir != "docker" {
		err = os.Chdir(dir)

	}
	if err == nil {
		err = io.StoreApmcfg(usr, fileWriter, dir)
	}
	io.ReplyEither(w, err, io.Grey+"using "+dir)

	apm := newApm(dir)
	apm.useErr = err
	return w.String(), apm
}

func newApm(dir string) *apm {
	return &apm{dir: dir, useErr: nil, unstaged: true, log: make([]string, 0), mu: sync.RWMutex{}}
}

func apmFlags(es es, apmUrl string, userFlags []string) []string {
	var add = func(flags map[string]string) []string {
		ret := make([]string, 0)
		for k, v := range flags {
			if v != "" {
				ret = append(ret, "-E", fmt.Sprintf(k, v))
			}
		}
		return ret
	}
	// if URL can't be parsed, apm-server won't start and log will show the cause
	URL, _ := url.Parse(apmUrl)
	flags := add(map[string]string{
		"apm-server.host=%s":               URL.Host,
		"output.elasticsearch.hosts=[%s]":  es.Url(),
		"output.elasticsearch.username=%s": es.username,
		"output.elasticsearch.password=%s": es.password,
	})
	return append(userFlags, append(flags, "-e")...)
}

// returns a client connected to an Elastic Search node with given `params`
// the string "last" loads from disk the last working params
// "local" is short for http://localhost:9200
// writes to disk
func elasticSearchUse(usr string, params ...string) (string, *es) {
	w := io.NewBufferWriter()
	if strcoll.Nth(0, params) == "last" {
		params = io.LoadEscfg(usr)
	}
	url := strcoll.Nth(0, params)
	if url == "local" {
		url = "http://localhost:9200"
	}
	username := strcoll.Nth(1, params)
	password := strcoll.Nth(2, params)
	client, err := elastic.NewClient(
		elastic.SetURL(url),
		elastic.SetBasicAuth(username, password),
		elastic.SetSniff(false),
	)
	if err == nil {
		io.StoreEscfg(usr, fileWriter, url, username, password)
	}
	io.ReplyEither(w, err, io.Grey+"using "+url)
	es := es{client, url, username, password, "hey-bench", err}
	return w.String(), &es
}

// deletes all apm-* indices
func reset(es *es) error {
	if es.useErr != nil {
		return es.useErr
	}
	_, err := es.Client.DeleteIndex("apm-*").Do(context.Background())
	if err == nil {
		_, err = es.Client.Flush("apm-*").Do(context.Background())
	}
	return err
}

// saves a report in the same elasticsearch instance used for tests
func indexReport(client *elastic.Client, prefix string, r api.TestReport) string {
	suffix := time.Now()
	bw := io.NewBufferWriter()
	_, err := client.Index().
		Index(fmt.Sprintf("%s-%d.%d.%d", prefix, suffix.Year(), suffix.Month(), suffix.Day())).
		Type("_doc").
		BodyJson(r).
		Do(context.Background())
	io.ReplyEitherNL(bw, err, "report indexed in elasticsearch")
	return bw.String()
}

// assumes posix
func maxRssUsed(cmd *exec.Cmd) int64 {
	// give time to the runtime to "fill in" process state
	time.Sleep(time.Second / 2)
	if cmd != nil {
		ps := cmd.ProcessState
		if ps != nil {
			var rusage syscall.Rusage
			syscall.Getrusage(syscall.RUSAGE_CHILDREN, &rusage)
			if runtime.GOOS == "linux" {
				return rusage.Maxrss * 1000
			} else if runtime.GOOS == "darwin" {
				return rusage.Maxrss
			}
		}
	}
	return 0
}

// returns the rss used before stopping the docker client
// that is not necessarily the max rss used, but in practical terms it will be
func stopDocker(w stdio.Writer) (int64, error) {
	// first get memory of the containerized process, then stop/remove the container
	sh := io.Shell(w, docker.Dir(), true)
	memStr, _ := sh("docker", "exec", "-i", docker.Container(), "ps", "-C", "apm-server", "h", "-o", "rss")
	mem, _ := strconv.ParseInt(memStr, 10, 64)
	// ps rss is in kb
	mem = mem * 1000
	_, err := sh("docker", "stop", docker.Container())
	return mem, err
}

// might silently fail, but is ok to miss some data here
func fillMissing(r api.TestReport, usr, rev, revDate string, mem, memLimit int64, flags []string) api.TestReport {
	// should find a better way to combine data made from bits known in LoadTest and bits known in EvalAndUpdate
	r.User = usr
	r.Revision = rev
	r.RevDate = revDate
	r.ApmFlags = s.Join(flags, " ")
	r.MaxRss = mem
	r.Limit = memLimit
	r.ReporterHost, _ = os.Hostname()
	selfDir := path.Join(os.Getenv("GOPATH"), "/src/github.com/elastic/hey-apm")
	if rRev, err := io.Shell(io.NewBufferWriter(), selfDir, false)("git", "rev-parse", "HEAD"); err != nil {
		r.ReporterRevision = rRev
	}
	return r
}
