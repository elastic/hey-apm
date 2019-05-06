[![Build Status](https://apm-ci.elastic.co/buildStatus/icon?job=apm-server/apm-hey-test-mbp/master)](https://apm-ci.elastic.co/job/apm-server/job/apm-hey-test-mbp/job/master)

Basic load generation for apm-server built on [hey](https://github.com/rakyll/hey).

# Requirements

hey-apm requires go modules support.  Tested with go1.12.1.

# Install

```
go get github.com/elastic/hey-apm
```

# Docker build

```
docker build -t hey-apm -f docker/Dockerfile .
```

# Interactive shell mode

To run hey-apm in interactive shell mode pass the `cli` flag without additional arguments.

```
./hey-apm -cli
```

You then can connect to the port 8234 and start sending commands. It is recommended to use a readline wrapper.

```
rlwrap telnet localhost 8234
```

### help

The `help` command describes the full semantics of the commands outlined above and some more things that hey-apm can do, like tail apm-server logs, etc.

# CI

The `Jenkinsfile` triggers sequentially:

- `unit-test.sh`

## Requirements
- [gvm](https://github.com/andrewkroh/gvm)

## Run scripts locally

```bash
  export WORKSPACE=`pwd`
  export APM_SERVER_DIR=<Path of the apm-server.git source code>
  export GO_VERSION=1.12.1
  ./scripts/jenkins/unit-test.sh
```

# Known issues

* Documentation and functionality is WIP
* Requires Elasticsearch 6.x
