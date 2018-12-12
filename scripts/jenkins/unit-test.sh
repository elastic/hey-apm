#!/usr/bin/env bash
set -exuo pipefail

export GOPATH=$WORKSPACE
export PATH=$PATH:$GOPATH/bin
eval "$(gvm ${GO_VERSION})"
echo "Installing hey-apm dependencies and running unit tests..."
go get -v -u github.com/golang/dep/cmd/dep
dep ensure -v

go get -v -u github.com/t-yuki/gocover-cobertura
go get -v -u github.com/jstemmer/go-junit-report

export COV_DIR="build/coverage"
export COV_FILE="${COV_DIR}/hey-apm.cov"
export OUT_FILE="build/test-report.out"
mkdir -p "${COV_DIR}"

(SKIP_EXTERNAL=1 SKIP_STRESS=1 go test -v ./... -coverprofile="${COV_FILE}" 2>&1 | tee ${OUT_FILE}) || echo -e "\033[31;49mTests FAILED\033[0m"

go-junit-report < ${OUT_FILE} > build/junit-hey-apm-report.xml
go tool cover -html="${COV_FILE}" -o "${COV_DIR}/coverage-hey-apm-report.html"
gocover-cobertura < "${COV_FILE}" > "${COV_DIR}/coverage-hey-apm-report.xml"
