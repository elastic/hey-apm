#!/usr/bin/env bash
set -exuo pipefail

export GOPATH=$WORKSPACE
export PATH=$PATH:$GOPATH/bin
eval "$(gvm 1.10.3)"
echo "Installing hey-apm dependencies and running unit tests..."
go get -v -u github.com/golang/dep/cmd/dep
go get -v -u github.com/graphaelli/hey/requester
go get -v -u github.com/olivere/elastic
go get -v -u github.com/pkg/errors
go get -v -u github.com/struCoder/pidusage
go get -v -u github.com/stretchr/testify/assert

go get -v -u github.com/t-yuki/gocover-cobertura
go get -v -u github.com/jstemmer/go-junit-report

dep ensure -v

export COV_DIR="build/coverage"
export COV_FILE="${COV_DIR}/hey-apm.cov"
export OUT_FILE="build/test-report.out"
mkdir -p "${COV_DIR}"

(SKIP_EXTERNAL=1 SKIP_STRESS=1 go test -v ./... -coverprofile="${COV_FILE}" 2>&1 | tee ${OUT_FILE}) || echo -e "\033[31;49mTests FAILED\033[0m"
go-junit-report < ${OUT_FILE} > build/junit-hey-apm-report.xml
go tool cover -html="${COV_FILE}" -o "${COV_DIR}/coverage-hey-apm-report.html"
gocover-cobertura < "${COV_FILE}" > "${COV_DIR}/coverage-hey-apm-report.xml"

