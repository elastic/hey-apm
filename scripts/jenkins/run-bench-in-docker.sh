#!/usr/bin/env bash
set -xeo pipefail

docker-compose version
docker-compose up --help

STACK_VERSION=${STACK_VERSION} \
ES_URL=${ES_URL} \
ES_AUTH=${ES_AUTH} \
USER_ID="$(id -u):$(id -g)" docker-compose \
  up \
  --no-color \
  --exit-code-from hey-apm \
  --build \
  --remove-orphans \
  --abort-on-container-exit \
  hey-apm

docker-compose down -v
