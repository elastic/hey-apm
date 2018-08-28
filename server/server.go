package server

import (
	"bufio"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	stdio "io"
	"log"
	"net"
	"os"
	"strconv"
	s "strings"

	"time"

	"github.com/elastic/hey-apm/server/api"
	"github.com/elastic/hey-apm/server/api/io"
	"github.com/elastic/hey-apm/server/client"
	"github.com/elastic/hey-apm/server/docker"
	"github.com/elastic/hey-apm/server/strcoll"
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
		go func(nConn net.Conn) {
			conn := client.WrapConnection(nConn)
			usr := resolveUsr(nConn)
			env := client.NewEvalEnvironment(usr)
			io.ReplyNL(conn, io.Grey+"Escape character doesn't work if you use rlwrap ¯\\_(ツ)_/¯.")
			io.ReplyEither(conn, io.Bootstrap(usr))
			io.ReplyNL(conn, "type 'help' for help")
			io.Prompt(conn)
			go env.EvalAndUpdate(usr, conn)
			reader := bufio.NewReader(conn)
			for {
				cmds, err := io.Read(env.Names())(reader.ReadString('\n'))
				logger.Println(cmds)
				if err != nil || len(cmds) == 0 {
					io.ReplyEitherNL(conn, err)
					io.Prompt(conn)
				}
				for _, cmd := range cmds {
					if strcoll.Nth(0, cmd) == "quit" || strcoll.Nth(0, cmd) == "exit" {
						conn.CancelSig.Broadcast()
						io.ReplyNL(conn, io.Grey+"bye!")
						conn.Close()
						return
					}
					out := eval(cmd, conn, env)
					if io.Reply(conn, out) {
						io.Prompt(conn)
					}
				}
			}
		}(c)
	}
}

// evaluates the given command `cmd`
// if `cmd` doesn't modify the evaluation environment state, it is evaluated right away
// otherwise it is pushed onto the connection's channel for it to be evaluated in a different goroutine
// the "cancel <cmdId>" command modifies the connection state by removing a command from its channel immediately
func eval(cmd []string, conn client.Connection, state api.State) string {
	fn := strcoll.Nth(0, cmd)
	switch {
	case fn == "help":
		return api.Help()
	case fn == "status":
		bw := api.Status(state)
		peekAndFilter(bw, "", conn)
		return bw.String()
	case fn == "define" && strcoll.Nth(2, cmd) == "":
		return api.NameDefinitions(state.Names(), strcoll.Nth(1, cmd))
	case fn == "cancel":
		if cmdId := strcoll.Nth(1, cmd); cmdId == "" {
			conn.CancelSig.Broadcast()
			return io.Grey + "ok\n"
		} else {
			bw := io.NewBufferWriter()
			peekAndFilter(bw, cmdId, conn)
			return bw.String()
		}
	case fn == "dump":
		return api.Dump(io.DiskWriter{}, strcoll.Nth(1, cmd), strcoll.Rest(2, cmd)...)
	case fn == "apm" && strcoll.Nth(1, cmd) == "tail":
		args, n := io.ParseCmdOption(cmd[2:], "-n", "", true)
		if n == "" {
			args, n = io.ParseCmdOption(args, "-", "10", false)
		}
		N, _ := strconv.Atoi(n)
		return api.Tail(state.ApmServer().Log(), N, strcoll.Nth(0, args))
	case fn == "apm" && strcoll.Nth(1, cmd) == "list":
		return docker.Images()
	case fn == "collate":
		// order is relevant
		args, ND := io.ParseCmdOption(cmd[1:], "-n", "20", true)
		args, sort := io.ParseCmdOption(args, "--sort", "report_date", true)
		args, opts := io.ParseCmdOptions(args)
		reports, err := state.ElasticSearch().FetchReports()
		if err == nil {
			return api.Collate(ND, sort, strcoll.Contains("csv", opts), args, reports)
		} else {
			return io.Red + err.Error()
		}
	case fn == "verify":
		reports, err := state.ElasticSearch().FetchReports()
		if err == nil {
			args, since := io.ParseCmdOption(cmd[1:], "-n", "168h", true)
			_, out := api.Verify(since, args, reports)
			return out
		} else {
			return io.Red + err.Error()
		}
	default:
		conn.EvalCh <- cmd
		time.Sleep(time.Second / 5)
		if len(conn.EvalCh) > 0 {
			return io.Grey + "(queued)\n"
		} else {
			return ""
		}
	}
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

// writes to `w` all commands queued in `conn`'s channel, unless a commands's id matches `cmdId`
// in that case, that command is removed from the queue
// other goroutines using the connection's channel must `wait`
func peekAndFilter(w stdio.Writer, cmdId string, conn client.Connection) {
	auxW := io.NewBufferWriter()
	conn.LockCh.Add(1)
	defer conn.LockCh.Done()
	queued := len(conn.EvalCh)
	for i := 0; i < queued; i++ {
		cmd := <-conn.EvalCh
		if shortHash(cmd) != cmdId {
			io.ReplyNL(auxW, fmt.Sprintf("%s %s %s %v", io.Magenta, shortHash(cmd), io.Grey, cmd))
			conn.EvalCh <- cmd
		}
	}
	io.ReplyNL(w, io.Grey+fmt.Sprintf("%d commands queued", len(conn.EvalCh)))
	io.Reply(w, auxW.String())
}

func shortHash(cmd []string) string {
	h := sha1.New()
	h.Write([]byte(s.Join(cmd, " ")))
	return hex.EncodeToString(h.Sum(nil))[:6]
}
