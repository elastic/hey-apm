#!/usr/bin/env bash
set -exuo pipefail

export GOPATH=$WORKSPACE
export PATH=$PATH:$GOPATH/bin

if [ ! -d "$APM_SERVER_DIR" ] ; then
  echo "you need to define APM_SERVER_DIR environment variable pointing to the APM server source code"
  exit 1
fi

eval "$(gvm 1.10.3)"
echo "Installing hey-apm dependencies and running unit tests..."
go get -v -u github.com/golang/dep/cmd/dep
go get -v -u github.com/graphaelli/hey/requester
go get -v -u github.com/olivere/elastic
go get -v -u github.com/pkg/errors
go get -v -u github.com/struCoder/pidusage
go get -v -u github.com/stretchr/testify/assert
echo "Fetching apm-server and installing latest go-licenser and mage..."
go get -v -u github.com/elastic/go-licenser
go get -v -u github.com/magefile/mage
(cd $GOPATH/src/github.com/magefile/mage && go run bootstrap.go)
echo "Running apm-server stress tests..."
set +x
ELASTICSEARCH_URL=$CLOUD_ADDR ELASTICSEARCH_USR=$CLOUD_USERNAME ELASTICSEARCH_PWD=$CLOUD_PASSWORD go test -timeout 2h  -v github.com/elastic/hey-apm/server/client