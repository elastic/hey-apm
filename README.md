[![Build Status](https://apm-ci.elastic.co/buildStatus/icon?job=apm-server/apm-hey-test-mbp/master)](https://apm-ci.elastic.co/job/apm-server/job/apm-hey-test-mbp/job/master)

Basic load generation for apm-server built on [hey](https://github.com/rakyll/hey).

# Install

```
# populate vendor/
go get github.com/golang/dep/cmd/dep
dep ensure -v
```

# Docker build

```
docker build -f docker/Dockerfile .
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
 
# Known issues

* Documentation and functionality is WIP 
* Requires Elasticsearch 6.x
