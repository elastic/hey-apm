#!/usr/bin/env bash
set -xeo pipefail

docker-compose version
docker-compose up --help

STACK_VERSION=${STACK_VERSION} USER_ID="$(id -u):$(id -g)" docker-compose \
  up \
  --exit-code-from hey-apm \
  --build \
  --remove-orphans \
  --abort-on-container-exit \
  hey-apm

docker-compose down -v
