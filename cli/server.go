package cli

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"sort"
	s "strings"
	"sync"
	"time"

	"github.com/elastic/hey-apm/cli/fileio"
	"github.com/elastic/hey-apm/commands"
	"github.com/elastic/hey-apm/es"
	"github.com/elastic/hey-apm/out"
	"github.com/elastic/hey-apm/target"
	"github.com/pkg/errors"

	"github.com/elastic/hey-apm/util"
)

// listens for connections on tcp:8234
func Serve() {

	logger := log.New(os.Stderr, "", log.Ldate|log.Ltime|log.Lshortfile)
	server, err := net.Listen("tcp", ":8234")
	if err != nil {
		logger.Println(err)
		os.Exit(1)
	}

	var connectionsC = make(chan net.Conn)
	go func() {
		for {
			conn, err := server.Accept()
			if err != nil {
				logger.Println(err)
				os.Exit(1)
			}
			connectionsC <- conn
		}
	}()

	for c := range connectionsC {
		go func(conn net.Conn) {

			env := newEnv(conn)
			out.ReplyEither(env, fileio.Bootstrap(env.usr))
			out.ReplyNL(env, "type 'help' for help")
			out.Prompt(env)

			go env.evalAsyncLoop()

			reader := bufio.NewReader(env)
			for {
				cmds, err := read(env.names())(reader.ReadString('\n'))
				logger.Println(cmds)
				if err != nil || len(cmds) == 0 {
					out.ReplyEitherNL(env, err)
					out.Prompt(env)
				}

				for _, cmd := range cmds {
					if util.Get(0, cmd) == "quit" || util.Get(0, cmd) == "exit" {
						env.cancel.Broadcast()
						out.ReplyNL(env, out.Grey+"bye!")
						env.Close()
						return
					}
					res := eval(cmd, env)
					if out.Reply(env, res) {
						out.Prompt(env)
					}
				}
			}
		}(c)
	}
}

// evaluates the given command `cmd`
// if `cmd` doesn't modify the state, it is evaluated right away
// otherwise it is pushed onto the env's channel for it to be evaluated in a different goroutine
func eval(cmd []string, env *env) string {
	fn := util.Get(0, cmd)

	switch {
	case fn == "help":
		return help()

	case fn == "status":
		return status(env.reportEs, env.testEs, env.apm, len(env.ch))

	case fn == "define" && util.Get(2, cmd) == "":
		return nameDefinitions(env.nameDefs, util.Get(1, cmd))

	case fn == "cancel":
		if subCmd := util.Get(1, cmd); subCmd == "current" {
			env.cancel.Broadcast()
		} else if subCmd == "queue" {
			func() {
				env.wg.Add(1)
				defer env.wg.Done()
				for i := 0; i < len(env.ch); i++ {
					<-env.ch
				}
			}()
		}
		return out.Grey + "ok/n"

	case fn == "dump":
		n, err := commands.Dump(out.FileWriter{Filename: util.Get(1, cmd)}, util.From(2, cmd)...)
		bw := out.NewBufferWriter()
		out.ReplyEitherNL(bw, err, out.Grey+util.ByteCountDecimal(int64(n))+" written to disk")
		return bw.String()

	case fn == "collate":
		if env.reportEs.Err != nil {
			return out.Red + env.reportEs.Err.Error()
		}
		args, size := util.ParseCmdIntOption(cmd[1:], "-n", 20)
		args, since := util.ParseCmdDurationOption(cmd[1:], "--since", time.Hour*24*7)
		args, sort := util.ParseCmdStringOption(args, "--sort", "report_date")
		reports, err := env.reportEs.FetchReports()
		if err == nil {
			return commands.Collate(size, since, sort, args, reports)
		} else {
			return out.Red + err.Error()
		}

	default:
		env.ch <- cmd
		time.Sleep(time.Second / 5)
		if len(env.ch) > 0 {
			return out.Grey + "(queued)\n"
		} else {
			return ""
		}
	}
}

type env struct {
	io.ReadWriteCloser
	// used for commands that modify the environment state
	ch chan []string
	// used ch synchronization
	wg *sync.WaitGroup
	// signal to abort running load tests triggered by user, connection termination, or apm-server termination
	cancel *sync.Cond
	// connected client IP
	usr string
	// elasticsearch cluster for querying and reporting data
	reportEs es.ReportNode
	// elasticsearch cluster for indexing apm data
	testEs es.TestNode
	// apm-server data
	apm apm
	// user name definitions for grouping commands, aliases, etc
	nameDefs map[string][]string
}

// loads from disk user-defined names
// the evaluation environment models everything that impacts the evaluation of user's commands,
// other than the commands themselves
func newEnv(conn net.Conn) *env {
	usr := resolveUsr(conn)
	env := env{
		ReadWriteCloser: conn,
		ch:              make(chan []string, 1000),
		wg:              &sync.WaitGroup{},
		cancel:          &sync.Cond{L: &sync.Mutex{}},
		usr:             usr,
		reportEs:        es.ReportNode{Err: errors.New("report node not specified")},
		testEs:          es.TestNode{Err: errors.New("test node not specified")},
		apm:             apm{},
		nameDefs:        fileio.LoadDefs(usr),
	}
	return &env
}

func (env env) wait() {
	env.cancel.L.Lock()
	defer env.cancel.L.Unlock()
	env.cancel.Wait()
}

func (env env) names() map[string][]string {
	return util.Copy(env.nameDefs)
}

// pulls commands from the env's channel and evaluates them sequentially (ie: blocking its goroutine)
// if a command is valid, its evaluation modifies the state
func (env *env) evalAsyncLoop() {
	for cmd := range env.ch {
		// out and err is what gets written to the TCP connection
		var err error
		res := "ok"

		fn := util.Get(0, cmd)
		arg1 := util.Get(1, cmd)
		arg2 := util.Get(2, cmd)
		args1 := util.From(1, cmd)
		args2 := util.From(2, cmd)
		args3 := util.From(3, cmd)

		switch {

		case fn == "define":
			fw := out.FileWriter{Filename: filepath.Join(fileio.BaseDir(), env.usr, "/.defs")}
			env.nameDefs, err = define(fw, args1, env.names())

		case fn == "es" && arg1 == "report" && arg2 == "use":
			env.reportEs = es.NewReportNode(args3...)
			err = env.reportEs.Err

		case fn == "es" && arg1 == "test" && arg2 == "use":
			env.testEs = es.NewTestNode(args3...)
			err = env.testEs.Err

		case fn == "es" && arg1 == "test" && arg2 == "reset":
			err = env.testEs.Reset()

		case fn == "apm" && arg1 == "use":
			env.apm = newApm(env, arg2, args3...)
			err = env.apm.err

		case fn == "apm" && arg1 == "restart":
			if env.apm.managed {
				if env.apm.proc != nil {
					apmStop(env.apm.proc)
				}
				env.apm.proc, err = apmStart(env, env.cancel.Broadcast, args2)
				if env.apm.proc != nil {
					res = fmt.Sprintf("apm-server process started with %d", env.apm.proc.Cmd.Process.Pid)
				}
			} else {
				err = errors.New("apm-server not managed")
			}

		case fn == "apm" && arg1 == "stop":
			err = apmStop(env.apm.proc)
			env.apm.proc = nil

		case fn == "apm" && arg1 == "tail":
			_, tailSize := util.ParseCmdIntOption(args2, "-n", 10)
			if env.apm.proc != nil {
				res = tail(env.apm.proc.log, tailSize)
			} else {
				res = "apm-server not running"
			}

		case fn == "test":
			if env.apm.err != nil {
				err = env.apm.err
				break
			}
			if env.apm.managed && env.apm.proc == nil {
				err = errors.New("apm server not started")
				break
			}
			if _, err = env.testEs.Health(); err != nil {
				break
			}
			if err = env.reportEs.Err; err != nil {
				break
			}
			var target target.Target
			target, err = makeTarget(env.apm.urls, args1...)
			if err != nil {
				break
			}

			docsBefore := env.testEs.Count()
			args1, labels := util.ParseCmdStringOption(args1, "--labels", "")
			args1, cooldown := util.ParseCmdDurationOption(args1, "--cooldown", time.Second)

			result := commands.LoadTest(env.ReadWriteCloser, env.wait, cooldown, target)

			report := commands.NewReport(
				target,
				result,
				labels,
				env.testEs.Url,
				util.Get(0, env.apm.urls),
				env.apm.version,
				len(env.apm.urls),
				env.testEs.Count()-docsBefore,
			)

			if report.Error == nil {
				report.Error = env.reportEs.IndexReport(report)
			}

			bw := out.NewBufferWriter()
			out.ReplyEitherNL(bw, report.Error, "saved report to Elasticsearch")
			res = bw.String()

		default:
			err = errors.New(fmt.Sprintf("nothing done for %s", s.Join(cmd, " ")))
		}

		out.ReplyEitherNL(env, err, res)
		out.Prompt(env)
		env.wg.Wait()
	}
}

// builds a target describing how to make requests to apm-server
// test duration, number of transactions, spans and frames are unnamed required args
func makeTarget(urls []string, args ...string) (target.Target, error) {
	duration := util.Get(0, args)
	args, throttle := util.ParseCmdIntOption(args, "--throttle", 32767)
	args, pause := util.ParseCmdDurationOption(args, "--pause", time.Millisecond*100)
	args, errorEvents := util.ParseCmdIntOption(args, "--errors", 0)
	args, agents := util.ParseCmdIntOption(args, "--agents", 1)
	args, stream := util.ParseCmdBoolOption(args, "--stream")
	args, reqTimeout := util.ParseCmdDurationOption(args, "--timeout", time.Second*10)
	t, err := target.NewTargetFromOptions(
		urls,
		target.RunTimeout(duration),
		target.NumAgents(agents),
		target.Throttle(throttle),
		target.Pause(pause),
		target.RequestTimeout(reqTimeout),
		target.Stream(stream),
		target.NumErrors(errorEvents),
		target.NumTransactions(util.Get(1, args)),
		target.NumSpans(util.Get(2, args)),
		target.NumFrames(util.Get(3, args)),
	)
	if err == nil && t.Config.RunTimeout < time.Second {
		err = errors.New("run timeout is too short")
	}
	return *t, err
}

func status(esReportClient es.ReportNode, esTestClient es.TestNode, apm apm, cmds int) string {
	w := out.NewBufferWriter()
	if esReportClient.Url == "" {
		out.ReplyNL(w, out.Red+"ElasticSearch report URL not specified")
	} else {
		out.ReplyNL(w, out.Green+fmt.Sprintf("ElasticSearch report URL: %s", esReportClient.Url))
	}

	if esTestClient.Url == "" {
		out.ReplyNL(w, out.Red+"ElasticSearch test URL not specified")
	} else {
		health, err := esTestClient.Health()
		out.ReplyEitherNL(w, err, out.Green+fmt.Sprintf("ElasticSearch test URL: %s [%s, %d docs]",
			esTestClient.Url, health, esTestClient.Count()))
	}

	if apm.managed == true {
		running := "not"
		if apm.proc != nil {
			running = ""
		}
		out.ReplyNL(w, out.Green+fmt.Sprintf("APM Server managed, version: %s [%s running]", apm.version, running))
	} else if len(apm.urls) == 0 {
		out.ReplyNL(w, out.Red+"APM Server URL not specified")
	} else {
		out.ReplyEitherNL(w, apm.err, out.Green+fmt.Sprintf("APM Server URL: %s [%s]", apm.urls, apm.version))
	}

	out.ReplyNL(w, out.Cyan+fmt.Sprintf("%d commands in the queue", cmds))
	return w.String()
}

func help() string {
	w := out.NewBufferWriter()
	out.ReplyNL(w, out.Yellow+"commands might be entered separated by semicolons, (eg: \"apm use last ; status\")")
	out.ReplyNL(w, out.Magenta+"status")
	out.ReplyNL(w, out.Grey+"    shows elasticsearch and apm-server current status, and number of queued commands")
	out.ReplyNL(w, out.Magenta+"es report use [<url> <username> <password> | local]")
	out.ReplyNL(w, out.Grey+"    connects to an elasticsearch node used for reporting, with given parameters")
	out.ReplyNL(w, out.Magenta+"        local"+out.Grey+" short for http://localhost:9200")
	out.ReplyNL(w, out.Magenta+"es test use [<url> <username> <password> | local]")
	out.ReplyNL(w, out.Grey+"    connects to an elasticsearch node used for testing, with given parameters")
	out.ReplyNL(w, out.Grey+"    NOTE: it must be the same one that apm-server is pointing to")
	out.ReplyNL(w, out.Magenta+"        local"+out.Grey+" short for http://localhost:9200")
	out.ReplyNL(w, out.Magenta+"elasticsearch reset")
	out.ReplyNL(w, out.Grey+"    deletes all the apm-* indices")
	out.ReplyNL(w, out.Magenta+"apm use <version> [<urls> | local]")
	out.ReplyNL(w, out.Grey+"    inform what apm-server to use and its version (release, tag, branch or commit)")
	out.ReplyNL(w, out.Magenta+"        local"+out.Grey+" checkouts a version and runs make on an GOPATH/src/github.com/elastic/apm-server")
	out.ReplyNL(w, out.Magenta+"        <urls>"+out.Grey+" several urls can be passed and hey-apm will RR request to them")
	out.ReplyNL(w, out.Magenta+"apm restart [<flags>]")
	out.ReplyNL(w, out.Grey+"    (re)starts a local apm-server process with given flags")
	out.ReplyNL(w, out.Magenta+"apm stop")
	out.ReplyNL(w, out.Grey+"    stops a local apm-server process started in the current hey-apm session")
	out.ReplyNL(w, out.Magenta+"apm tail [-n <n>]")
	out.ReplyNL(w, out.Grey+"    shows the last lines of the apm server log")
	out.ReplyNL(w, out.Magenta+"        -n <n>"+out.Grey+" shows the last <n> lines up to 1000, defaults to 10")
	out.ReplyNL(w, out.Magenta+"dump <file_name> <errors> <transactions> <spans> <frames>")
	out.ReplyNL(w, out.Grey+"    writes to <file_name> a payload with the given profile (described above)")
	out.ReplyNL(w, out.Magenta+"test <duration> <transactions> <spans> <frames> [<OPTIONS>...]")
	out.ReplyNL(w, out.Grey+"    creates a workload for testing apm-server")
	out.ReplyNL(w, out.Magenta+"        <duration>"+out.Grey+" duration of the load test (eg \"1m\")")
	out.ReplyNL(w, out.Magenta+"        <transactions>"+out.Grey+" transactions per request body")
	out.ReplyNL(w, out.Magenta+"        <spans>"+out.Grey+" spans per transaction")
	out.ReplyNL(w, out.Magenta+"        <frames>"+out.Grey+" frames per document (for spans and errors)")
	out.ReplyNL(w, out.Grey+"    OPTIONS:")
	out.ReplyNL(w, out.Magenta+"        --errors <errors>"+out.Grey+" number of errors per request body")
	out.ReplyNL(w, out.Grey+"        defaults to 0")
	out.ReplyNL(w, out.Magenta+"        --stream"+out.Grey+" use the streaming protocol")
	out.ReplyNL(w, out.Magenta+"        --agents <agents>"+out.Grey+" number of simultaneous agents sending queries")
	out.ReplyNL(w, out.Grey+"        defaults to 1")
	out.ReplyNL(w, out.Magenta+"        --throttle <throttle>"+out.Grey+" upper limit of queries per second to send")
	out.ReplyNL(w, out.Grey+"        defaults to 1")
	out.ReplyNL(w, out.Magenta+"        --timeout <timeout>"+out.Grey+" client request timeout")
	out.ReplyNL(w, out.Grey+"        defaults to 10s")
	out.ReplyNL(w, out.Magenta+"        --cooldown <cooldown>"+out.Grey+" time to wait after sending requests and before stopping the apm-server")
	out.ReplyNL(w, out.Grey+"        defaults to 1s")
	out.ReplyNL(w, out.Magenta+"        --labels <labels>"+out.Grey+" any labels with the format k1=v1,k2=v2")
	out.ReplyNL(w, out.Magenta+"cancel current|queue")
	out.ReplyNL(w, out.Magenta+"         current"+out.Grey+" cancels the ongoing test")
	out.ReplyNL(w, out.Magenta+"         queue"+out.Grey+" cancels all the queued commands")
	out.ReplyNL(w, out.Magenta+"collate <VARIABLE> [-n <n> --since <since> --sort <CRITERIA> <FILTER>...]")
	out.ReplyNL(w, out.Grey+"    queries reports generated by workload tests, and per each result shows other reports in which only VARIABLE is different")
	out.ReplyNL(w, out.Magenta+"        -n <n>"+out.Grey+" shows up to <n> report groups")
	out.ReplyNL(w, out.Grey+"    defaults to 20")
	out.ReplyNL(w, out.Magenta+"        --since <since>"+out.Grey+" filters out reports older than <since>")
	out.ReplyNL(w, out.Grey+"    defaults to 1 week")
	out.ReplyNL(w, out.Magenta+"        --sort <CRITERIA>"+out.Grey+" sorts the results by the given CRITERIA, defaults to report_date")
	out.ReplyNL(w, out.Grey+"        CRITERIA:")
	out.ReplyNL(w, out.Magenta+"                report_date"+out.Grey+" date of the generated report, most recent first")
	out.ReplyNL(w, out.Magenta+"                duration"+out.Grey+" duration of the workload test, higher first")
	out.ReplyNL(w, out.Magenta+"                pushed_volume"+out.Grey+" bytes pushed per second, higher first")
	out.ReplyNL(w, out.Magenta+"                throughput"+out.Grey+" indexed documents per second, higher first")
	out.ReplyNL(w, out.Magenta+"        <VARIABLE>"+out.Grey+" shows together reports generated from workload tests with the same parameters except VARIABLE")
	out.ReplyNL(w, out.Grey+"        VARIABLE:")
	out.ReplyNL(w, out.Magenta+"                duration"+out.Grey+" duration of the test")
	out.ReplyNL(w, out.Magenta+"                transactions"+out.Grey+" transactions per request body")
	out.ReplyNL(w, out.Magenta+"                errors"+out.Grey+" errors per request body")
	out.ReplyNL(w, out.Magenta+"                spans"+out.Grey+" spans per transaction")
	out.ReplyNL(w, out.Magenta+"                frames"+out.Grey+" frames per document")
	out.ReplyNL(w, out.Magenta+"                stream"+out.Grey+" whether a test used the streaming protocol or not")
	out.ReplyNL(w, out.Magenta+"                agents"+out.Grey+" number of concurrent agents")
	out.ReplyNL(w, out.Magenta+"                throttle"+out.Grey+" upper limit of queries per second sent")
	out.ReplyNL(w, out.Magenta+"                timeout"+out.Grey+" request timeout")
	out.ReplyNL(w, out.Magenta+"                apm_version"+out.Grey+" git version and commit (if the version is variable, the revision necessarily varies too)")
	out.ReplyNL(w, out.Magenta+"                apm_host"+out.Grey+" apm hostname(s) separated by ',' ordered alphabetically")
	out.ReplyNL(w, out.Magenta+"                apms"+out.Grey+" number of apm servers running")
	out.ReplyNL(w, out.Magenta+"        <FILTER>"+out.Grey+" returns only reports matching all given filters, specified like <field>=|!=|<|><value>")
	out.ReplyNL(w, out.Grey+"        dates must be formatted like \"2018-28-02\" and durations like \"1m\"")
	out.ReplyNL(w, out.Magenta+"help")
	out.ReplyNL(w, out.Grey+"    shows this help")
	out.ReplyNL(w, out.Magenta+"quit")
	out.ReplyNL(w, out.Grey+"    quits this connection")
	out.ReplyNL(w, out.Magenta+"exit")
	out.ReplyNL(w, out.Grey+"    same as quit")
	return w.String()
}

// returns formatted name definitions containing `match` in either left or right side, or all if `match` is empty
func nameDefinitions(nameDefs map[string][]string, match string) string {
	w := out.NewBufferWriter()
	if len(nameDefs) == 0 {
		out.ReplyNL(w, out.Grey+"nothing to show")
		return w.String()
	}
	keys := make([]string, 0)
	for k := range nameDefs {
		keys = append(keys, k)
	}
	sort.Sort(sort.StringSlice(keys))
	for _, k := range keys {
		v := nameDefs[k]
		cmd := s.Join(v, " ")
		if match == "" || s.Contains(k, match) || s.Contains(cmd, match) {
			out.ReplyNL(w, out.Magenta+k+out.Grey+" "+cmd)
		}
	}
	return w.String()
}

// defines a name and returns a new name definitions map
// a name definition maps 1 word to several words
// cmd examples:
// `ma apm switch master` maps `ma` to `apm switch master`
// `fe -E apm-server.frontend=true` maps `fe` to `-E apm-server.frontend=true`
// `mafe ma ; test 1s 1 1 1 1 fe` (invoking `mafe` will run `apm switch master`, then `test 1s 1 1 1 1 -E apm-server.frontend=true`)
// `rm fe` (will remove the `fe` definition and cause `mafe` invocations to fail)
func define(w io.Writer, cmd []string, nameDefs map[string][]string) (map[string][]string, error) {
	var err error
	m := util.Copy(nameDefs)
	left, right := util.Get(0, cmd), util.From(1, cmd)
	if left == "rm" {
		// this might leave dangling names
		delete(m, util.Get(0, right))
		err = fileio.StoreDefs(w, m)
	} else {
		if util.Contains(left, right) {
			err = errors.New(left + " can't appear in the right side")
		} else {
			m[left] = right
			err = fileio.StoreDefs(w, m)
		}
	}
	return m, err
}

// a user is determined by its hostname or ip address
// users are modeled so to persist data across sessions for them
func resolveUsr(conn net.Conn) string {
	conn.RemoteAddr().String()
	addr, _, _ := net.SplitHostPort(conn.RemoteAddr().String())
	hosts, _ := net.LookupAddr(addr)
	if len(hosts) == 0 {
		return addr
	}
	return hosts[0]
}
