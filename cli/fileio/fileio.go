package fileio

import (
	"bytes"
	stdio "io"
	"io/ioutil"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/elastic/hey-apm/util"
	"github.com/pkg/errors"
)

func BaseDir() string {
	home := os.Getenv("HOME")
	if usr, err := user.Current(); home == "" && err != nil {
		home = usr.HomeDir
	}
	return filepath.Join(home, ".heydata/")
}

// creates a data directory for given user with empty .escfg, .apmcfg and .defs and files
// used to keep configuration across sessions
func Bootstrap(usr string) error {
	if bootstrapped(usr) {
		return nil
	}
	err := onConfigFiles(func(f string) error {
		_, err := os.Create(filepath.Join(BaseDir(), usr, "/", f))
		return err
	})
	return errors.Wrap(err, "**NOTE**: data generated in this session might not be persisted")
}

func LoadDefs(usr string) map[string][]string {
	defs := make(map[string][]string)
	b, _ := ioutil.ReadFile(filepath.Join(BaseDir(), usr, "/.defs"))
	if len(b) > 0 {
		for _, line := range strings.Split(string(b), "\n") {
			token := strings.Fields(line)
			if len(util.From(1, token)) > 0 {
				defs[token[0]] = token[1:]
			}
		}
	}
	return defs
}

func StoreDefs(w stdio.Writer, defs map[string][]string) error {
	var buf bytes.Buffer
	for k, v := range defs {
		buf.WriteString(k)
		buf.WriteString(" ")
		buf.WriteString(strings.Join(v, " "))
		buf.WriteString("\n")
	}
	_, err := w.Write(buf.Bytes())
	return err
}

func bootstrapped(usr string) bool {
	os.Mkdir(filepath.Join(BaseDir(), usr), 0700)
	return onConfigFiles(func(f string) error {
		_, err := os.Stat(filepath.Join(BaseDir(), usr, "/", f))
		return err
	}) == nil
}

// TODO simplify
func onConfigFiles(fn func(string) error) error {
	var err error
	for _, f := range []string{".defs"} {
		if err != nil {
			continue
		}
		err = fn(f)
	}
	return err
}
