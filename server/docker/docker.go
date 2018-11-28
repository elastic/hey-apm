package docker

import (
	"github.com/elastic/hey-apm/server/api/io"
	"os"
	"path/filepath"
)

func Image(branch, revision string) string {
	if revision != "" {
		return Container() + "-" + branch + ":" + revision
	} else {
		return Container() + "-" + branch + ":latest"
	}
}

func Images() string {
	bw := io.NewBufferWriter()
	sh := io.Shell(bw, Dir(), true)
	sh("docker", "images", "--filter", "label="+Label())
	return bw.String()
}

func Label() string {
	return "for=hey"
}

func Dir() string {
	return filepath.Join(os.Getenv("GOPATH"), "src/github.com/elastic/hey-apm/server/docker")
}

func Container() string {
	return "heyapmserver"
}
