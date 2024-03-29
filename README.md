[![Build Status](https://apm-ci.elastic.co/buildStatus/icon?job=apm-server/apm-hey-test-mbp/main)](https://apm-ci.elastic.co/job/apm-server/job/apm-hey-test-mbp/job/main)

# Overview

hey-apm is a basic load generation tool for apm-server simulating different workloads.
Back in the intake V1 days it was based on [hey](https://github.com/rakyll/hey),
but now it uses the Go APM agent to generate events.

hey-apm generates performance reports that can be indexed in Elasticsearch.
It can be used manually or automatically (ie. in a CI environment)

# Requirements

hey-apm requires go modules support.  Tested with go1.14.4.

# Install

```
go get github.com/elastic/hey-apm
```

# Docker build

```
docker build -t hey-apm -f docker/Dockerfile .
```

### Usage

Run `./hey-apm -help` or see `main.go`

# CI

The `Jenkinsfile` triggers sequentially:

- `.ci/scripts/unit-test.sh`
- `.ci/scripts/run-bench-in-docker.sh`

## Requirements
- [gvm](https://github.com/andrewkroh/gvm)

## Run scripts locally

```bash
  ./.ci/scripts/unit-test.sh 1.14
```

# How to run locally the hey-apm using a docker-compose services

Run `.ci/scripts/run-bench-in-docker.sh`

## Configure the ES stack version

Run `ELASTIC_STACK=<version> .ci/scripts/run-bench-in-docker.sh`

## Configure the docker image for the ES stack

Run `APM_DOCKER_IMAGE=<docker-image> .ci/scripts/run-bench-in-docker.sh`

## Configure the ES stack where to send the metrics to

Run `ELASTIC_STACK=<version> ES_URL=<url> ES_USER=<user> ES_PASS=<password> .ci/scripts/run-bench-in-docker.sh`

# Known issues

* A single Go agent (as hey-apm uses) can't push enough load to overwhelm the apm-server,
as it will drop data too conservatively for benchmarking purposes.
