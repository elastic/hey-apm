package docker

import (
	"os"
	"path/filepath"
	"strconv"

	"github.com/elastic/hey-apm/server/api"
	"github.com/elastic/hey-apm/server/api/io"
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

func IsDockerized(apm api.ApmServer) bool {
	return apm.Dir() == "docker"
}

// s has the format used by docker --memory flag
func ToBytes(s string) int64 {
	if len(s) < 2 {
		return parseInt(s, 1)
	}
	val, unit := s[:len(s)-1], s[len(s)-1:]
	switch unit {
	case "b":
		return parseInt(val, 1)
	case "k":
		return parseInt(val, 1000)
	case "m":
		return parseInt(val, 1000*1000)
	case "g":
		return parseInt(val, 1000*1000*1000)
	default:
		return parseInt(s, 1)
	}
}

func parseInt(s string, mult int64) int64 {
	i, _ := strconv.ParseInt(s, 10, 0)
	return i * mult
}
