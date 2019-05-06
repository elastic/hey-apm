#!/usr/bin/env bash
# Requirements
#   env variable WORKSPACE
#   env variable APM_SERVER_DIR
#   env variable CLOUD_ADDR
#   env variable CLOUD_USERNAME
#   env variable CLOUD_PASSWORD
#
set -euo pipefail

RED='\033[31;49m'
GREEN='\033[32;49m'
NC='\033[0m' # No Color

export GOPATH=$WORKSPACE/build
export PATH=$PATH:$GOPATH/bin

if [ ! -d "$APM_SERVER_DIR" ] ; then
  echo "you need to define APM_SERVER_DIR environment variable pointing to the APM server source code"
  exit 1
fi

eval "$(gvm use ${GO_VERSION})"

echo "Installing hey-apm dependencies"
go get -v -u github.com/t-yuki/gocover-cobertura
go get -v -u github.com/jstemmer/go-junit-report

echo "Running apm-server stress tests..."

export COV_DIR="build/coverage"
export COV_FILE="${COV_DIR}/hey-apm-stress-test.cov"
export OUT_FILE="build/stress-test.out"
mkdir -p "${COV_DIR}"

(ELASTICSEARCH_URL=$CLOUD_ADDR \
  ELASTICSEARCH_USR=$CLOUD_USERNAME \
  ELASTICSEARCH_PWD=$CLOUD_PASSWORD \
  go test -timeout 2h  -v github.com/elastic/hey-apm/server/client 2>&1 | tee ${OUT_FILE}) \
  && echo -e "${GREEN}Tests PASSED${NC}" || echo -e "${RED}Tests FAILED${NC}"

go-junit-report < ${OUT_FILE} > build/junit-hey-apm-stress-test-report.xml
