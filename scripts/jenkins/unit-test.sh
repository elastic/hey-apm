#!/usr/bin/env bash
# Requirements
#   env variable WORKSPACE
#   env variable GO_VERSION

set -euo pipefail

RED='\033[31;49m'
GREEN='\033[32;49m'
NC='\033[0m' # No Color

echo "Setup Go ${GO_VERSION}"
export GOPATH=${WORKSPACE}/build
export PATH=$PATH:$GOPATH/bin
eval "$(gvm ${GO_VERSION})"

export COV_DIR="build/coverage"
export COV_FILE="${COV_DIR}/hey-apm.cov"
export OUT_FILE="build/test-report.out"
mkdir -p "${COV_DIR}"

echo "Running unit tests..."
(SKIP_EXTERNAL=1 SKIP_STRESS=1 go test -v ./... -coverprofile="${COV_FILE}" 2>&1 | tee ${OUT_FILE}) \
  && echo -e "${GREEN}Tests PASSED${NC}" || echo -e "${RED}Tests FAILED${NC}"
go-junit-report < ${OUT_FILE} > build/junit-hey-apm-report.xml

echo "Running cobertura"
go tool cover -html="${COV_FILE}" -o "${COV_DIR}/coverage-hey-apm-report.html" \
  && echo -e "${GREEN}Tests PASSED${NC}" || echo -e "${RED}Tests FAILED${NC}"

gocover-cobertura < "${COV_FILE}" > "${COV_DIR}/coverage-hey-apm-report.xml"
