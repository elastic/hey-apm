#!/usr/bin/env bash
# Stress testing the given Go version and APM server
#
# NOTE: It's required to be launched inside the root of the project.
#
# Usage: ./.ci/scripts/run-test.sh 1.14 /src/apm-server
#
# Requirements
#   env variable CLOUD_ADDR
#   env variable CLOUD_USERNAME
#   env variable CLOUD_PASSWORD
#
set -euo pipefail

RED='\033[31;49m'
GREEN='\033[32;49m'
NC='\033[0m' # No Color

GO_VERSION=${1:?Please specify the Go version}
APM_SERVER_DIR=${2:?Please specify the path pointing to the APM server source code}

echo "Setup Go ${GO_VERSION}"
GOPATH=$(pwd)/build
export PATH=$PATH:$GOPATH/bin
eval "$(gvm "${GO_VERSION}")"

export COV_DIR="build/coverage"
export COV_FILE="${COV_DIR}/hey-apm-stress-test.cov"
export OUT_FILE="build/stress-test.out"
mkdir -p "${COV_DIR}"

echo "Installing hey-apm dependencies for Jenkins..."
go get -v -u github.com/t-yuki/gocover-cobertura
go get -v -u github.com/jstemmer/go-junit-report

echo "Running apm-server stress tests..."
(ELASTICSEARCH_URL=$CLOUD_ADDR \
  ELASTICSEARCH_USR=$CLOUD_USERNAME \
  ELASTICSEARCH_PWD=$CLOUD_PASSWORD \
  go test -timeout 2h  -v github.com/elastic/hey-apm/server/client 2>&1 | tee ${OUT_FILE}) \
  && echo -e "${GREEN}Tests PASSED${NC}" || echo -e "${RED}Tests FAILED${NC}"

go-junit-report < ${OUT_FILE} > build/junit-hey-apm-stress-test-report.xml
