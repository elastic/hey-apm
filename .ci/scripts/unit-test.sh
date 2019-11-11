#!/usr/bin/env bash
#
# Test the given Go version.
#
# NOTE: It's required to be launched inside the root of the project.
#
# Usage: ./.ci/scripts/unit-test.sh 1.12.1
#
set -exuo pipefail

RED='\033[31;49m'
GREEN='\033[32;49m'
NC='\033[0m' # No Color

GO_VERSION=${1:?Please specify the Go version}

echo "Setup Go ${GO_VERSION}"
GOPATH=$(pwd)/build
export PATH=$PATH:$GOPATH/bin
eval "$(gvm "${GO_VERSION}")"
env | sort

export COV_DIR="build/coverage"
export COV_FILE="${COV_DIR}/hey-apm.cov"
export OUT_FILE="build/test-report.out"
mkdir -p "${COV_DIR}"

echo "Installing hey-apm dependencies for Jenkins..."
go get -v -u github.com/t-yuki/gocover-cobertura
go get -v -u github.com/jstemmer/go-junit-report

echo "Running unit tests..."
(SKIP_EXTERNAL=1 SKIP_STRESS=1 go test -v ./... -coverprofile="${COV_FILE}" 2>&1 | tee ${OUT_FILE}) \
  && echo -e "${GREEN}Tests PASSED${NC}" || echo -e "${RED}Tests FAILED${NC}"
go-junit-report < ${OUT_FILE} > build/junit-hey-apm-report.xml

echo "Running cobertura"
go tool cover -html="${COV_FILE}" -o "${COV_DIR}/coverage-hey-apm-report.html" \
  && echo -e "${GREEN}Tests PASSED${NC}" || echo -e "${RED}Tests FAILED${NC}"

gocover-cobertura < "${COV_FILE}" > "${COV_DIR}/coverage-hey-apm-report.xml"
