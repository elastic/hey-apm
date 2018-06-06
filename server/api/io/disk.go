package io

import (
	"bytes"
	"io/ioutil"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/elastic/hey-apm/server/strcoll"
	"github.com/pkg/errors"
)

type DiskWriter struct{}

func (_ DiskWriter) WriteToFile(filename string, content []byte) error {
	return ioutil.WriteFile(filename, content, 0644)
}

func chBaseDir() {
	base, err := createBaseDir()
	if err == nil {
		err = os.Chdir(base)
	}
	if err != nil {
		panic(err)
	}
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

func bootstrapped(usr string) bool {
	os.Mkdir(filepath.Join(BaseDir(), usr), 0700)
	return onConfigFiles(func(f string) error {
		_, err := os.Stat(filepath.Join(BaseDir(), usr, "/", f))
		return err
	}) == nil
}

func onConfigFiles(fn func(string) error) error {
	var err error
	for _, f := range []string{".escfg", ".apmcfg", ".defs"} {
		if err != nil {
			continue
		}
		err = fn(f)
	}
	return err
}

func LoadEscfg(usr string) []string {
	return loadCfg(usr, "/.escfg")
}

func LoadApmcfg(usr string) []string {
	return loadCfg(usr, "/.apmcfg")
}

func loadCfg(usr string, file string) []string {
	b, _ := ioutil.ReadFile(filepath.Join(BaseDir(), usr, file))
	if len(b) > 0 {
		return strings.Split(string(b), " ")
	}
	return []string{}
}

func StoreEscfg(usr string, writer FileWriter, escfg ...string) error {
	return storeCfg(usr, writer, "/.escfg", escfg...)
}

func StoreApmcfg(usr string, writer FileWriter, apmcfg ...string) error {
	return storeCfg(usr, writer, "/.apmcfg", apmcfg...)
}

func storeCfg(usr string, writer FileWriter, file string, escfg ...string) error {
	f := filepath.Join(BaseDir(), usr, file)
	return writer.WriteToFile(f, []byte(strings.Join(escfg, " ")))
}

func LoadDefs(usr string) map[string][]string {
	defs := make(map[string][]string)
	b, _ := ioutil.ReadFile(filepath.Join(BaseDir(), usr, "/.defs"))
	if len(b) > 0 {
		for _, line := range strings.Split(string(b), "\n") {
			token := strings.Fields(line)
			if len(strcoll.Rest(1, token)) > 0 {
				defs[token[0]] = token[1:]
			}
		}
	}
	return defs
}

func StoreDefs(usr string, writer FileWriter, defs map[string][]string) error {
	f := filepath.Join(BaseDir(), usr, "/.defs")
	var buf bytes.Buffer
	for k, v := range defs {
		buf.WriteString(k)
		buf.WriteString(" ")
		buf.WriteString(strings.Join(v, " "))
		buf.WriteString("\n")
	}
	return writer.WriteToFile(f, buf.Bytes())
}

func createBaseDir() (string, error) {
	d := BaseDir()
	var err error
	if _, err = os.Stat(d); err != nil {
		err = os.Mkdir(d, 0700)
	}
	return d, err
}

// exposed only for tests :(
func BaseDir() string {
	home := os.Getenv("HOME")
	if usr, err := user.Current(); home == "" && err != nil {
		home = usr.HomeDir
	}
	return filepath.Join(home, ".heydata/")
}
