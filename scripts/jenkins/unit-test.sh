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
dep ensure -v
SKIP_EXTERNAL=1 SKIP_STRESS=1 go test -v ./...