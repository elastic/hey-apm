#!/usr/bin/env bash
set -xeo pipefail

function finish {
  mkdir -p build
  {
    docker-compose version
    docker system info
    docker ps -a
    docker-compose logs apm-server hey-apm elasticsearch wait
  } > build/environment.txt
  docker-compose down -v
}
trap finish EXIT INT TERM

STACK_VERSION=${STACK_VERSION} \
ES_URL=${ES_URL} \
ES_USER=${ES_USER} \
ES_PASS=${ES_PASS} \
USER_ID="$(id -u):$(id -g)" docker-compose \
  up \
  --no-color \
  --exit-code-from hey-apm \
  --build \
  --remove-orphans \
  --abort-on-container-exit \
  hey-apm
