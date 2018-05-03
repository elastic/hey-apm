package io

import (
	"runtime"

	"github.com/pkg/errors"
)

func BootstrapChecks() {
	chBaseDir()
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		// meaning only linux and darwin have been somewhat tested
		panic(errors.New("only linux and darwin OS supported"))
	}
	sh := Shell(NewBufferWriter(), ".", true)
	sh("git", "--version")
	sh("go", "version")
	if _, err := sh(""); err != nil {
		panic(err)
	}

}
