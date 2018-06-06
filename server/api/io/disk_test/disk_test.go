package disk_test

import (
	"fmt"
	"os"
	"path/filepath"
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
		// can't defer this
		os.RemoveAll(filepath.Join(io.BaseDir(), "test_user"))
		os.Exit(code)
	} else {
		// run with test -v if you want to read this
		fmt.Println("skipping disk tests")
		os.Exit(0)
	}
}

func TestLoadStoreCfg(t *testing.T) {
	err := io.Bootstrap("test_user")
	assert.NoError(t, err)

	cfg := io.LoadEscfg("test_user")
	assert.Equal(t, []string{}, cfg)

	err = io.StoreEscfg("test_user", io.DiskWriter{}, "a", "b", "c")
	assert.NoError(t, err)

	cfg = io.LoadEscfg("test_user")
	assert.Equal(t, []string{"a", "b", "c"}, cfg)

	err = io.StoreEscfg("test_user", io.DiskWriter{})
	assert.NoError(t, err)

	cfg = io.LoadEscfg("test_user")
	assert.Equal(t, []string{}, cfg)
}

func TestLoadStoreDefs(t *testing.T) {
	err := io.Bootstrap("test_user")
	assert.NoError(t, err)

	defs := io.LoadDefs("test_user")
	assert.Equal(t, map[string][]string{}, defs)

	err = io.StoreDefs("test_user", io.DiskWriter{}, map[string][]string{"a": {"1", "2"}, "b": {"3"}})
	assert.NoError(t, err)

	defs = io.LoadDefs("test_user")
	assert.Equal(t, map[string][]string{"a": {"1", "2"}, "b": {"3"}}, defs)

	err = io.StoreDefs("test_user", io.DiskWriter{}, map[string][]string{})
	assert.NoError(t, err)

	defs = io.LoadDefs("test_user")
	assert.Equal(t, map[string][]string{}, defs)
}

func printWarning() {
	fmt.Println("\ndisk tests have external dependencies: read/write access to home directory")
	fmt.Println("you might disable external tests with SKIP_EXTERNAL=1 go test -v ./...")
	fmt.Println()
}
