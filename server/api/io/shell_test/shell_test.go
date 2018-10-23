package shell_test

import (
	"fmt"
	"os"
	"path"
	"testing"

	"github.com/elastic/hey-apm/server/api/io"
	"github.com/stretchr/testify/assert"
)

/*
Test in a different package so to use several TestMain's and provide granularity to rule out external tests without
adding any code to the tests themselves
*/
func TestMain(m *testing.M) {
	if os.Getenv("SKIP_EXTERNAL") == "" {
		code := m.Run()
		if code > 0 {
			printWarning()
		}
		os.Exit(code)
	} else {
		// run with test -v if you want to read this
		fmt.Println("skipping shell tests")
		os.Exit(0)
	}
}

type shell func(...string) (string, error)

func TestShellVerbose(t *testing.T) {
	bw := io.NewBufferWriter()
	home := os.Getenv("HOME")

	noVerbose := io.Shell(bw, home, false)
	_, err := noVerbose("ls")
	assert.NoError(t, err)
	assert.NotContains(t, bw.String(), io.Magenta)

	verbose := io.Shell(bw, home, true)
	_, err = verbose("ls")
	assert.NoError(t, err)
	assert.Contains(t, bw.String(), io.Magenta)
}

func TestShellError(t *testing.T) {
	newShell := func() shell {
		return io.Shell(io.NewBufferWriter(), os.Getenv("HOME"), false)
	}
	for _, test := range []struct {
		fns            func(shell) (string, error)
		hasOut, hasErr bool
	}{
		{
			func(sh shell) (string, error) {
				return sh("")
			},
			false,
			false,
		},
		{
			func(sh shell) (string, error) {
				sh("")
				return sh("ls")
			},
			true,
			false,
		},
		{
			func(sh shell) (string, error) {
				sh("ls")
				return sh("")
			},
			false,
			false,
		},
		{
			func(sh shell) (string, error) {
				sh("ls")
				return sh("%^&%$&$*@£")
			},
			false,
			true,
		},
		{
			func(sh shell) (string, error) {
				sh("%^&%$&$*@£")
				return sh("ls")
			},
			false,
			true,
		},
	} {
		out, err := test.fns(newShell())
		assert.Equal(t, test.hasOut, out != "")
		assert.Equal(t, test.hasErr, err != nil)
	}
}

func TestShellSelf(t *testing.T) {
	heyDir := path.Join(os.Getenv("GOPATH"), "/src/github.com/elastic/hey-apm")
	sh := io.Shell(nil, heyDir, false)
	out, err := sh("git", "rev-parse", "HEAD")
	assert.NoError(t, err)
	assert.Len(t, out, 40)
}

func printWarning() {
	fmt.Println("\nshell tests have external dependencies: HOME env variable set and a POSIX OS with read access to home")
	fmt.Println("you might disable external tests with SKIP_EXTERNAL=1 go test -v ./...")
	fmt.Println()
}
