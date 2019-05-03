#!/usr/bin/env bash
set -euo pipefail

# https://github.com/moovweb/gvm/issues/188
[[ -s "$GVM_ROOT/scripts/gvm" ]] && source "$GVM_ROOT/scripts/gvm"

echo "Setup Go"
export GOPATH=$WORKSPACE
export PATH=$PATH:$GOPATH/bin
eval "$(gvm use ${GO_VERSION})"

echo "Installing hey-apm dependencies"
go get -v -u github.com/t-yuki/gocover-cobertura
go get -v -u github.com/jstemmer/go-junit-report

echo "Running unit tests..."
export COV_DIR="build/coverage"
export COV_FILE="${COV_DIR}/hey-apm.cov"
export OUT_FILE="build/test-report.out"
mkdir -p "${COV_DIR}"

(SKIP_EXTERNAL=1 SKIP_STRESS=1 go test -v ./... -coverprofile="${COV_FILE}" 2>&1 | tee ${OUT_FILE}) || echo -e "\033[31;49mTests FAILED\033[0m"
go-junit-report < ${OUT_FILE} > build/junit-hey-apm-report.xml

echo "Running cobertura"
go tool cover -html="${COV_FILE}" -o "${COV_DIR}/coverage-hey-apm-report.html"
gocover-cobertura < "${COV_FILE}" > "${COV_DIR}/coverage-hey-apm-report.xml"
