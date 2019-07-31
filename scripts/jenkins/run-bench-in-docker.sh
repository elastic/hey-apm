#!/usr/bin/env bash
set -xeo pipefail

function finish {
  echo "***********************************************************"
  docker-compose version
  docker-compose up --help
  echo "***********************************************************"
  docker system info
  echo "***********************************************************"
  docker ps -a
  # list the services explicitly to report the logs by service for easy reading
  docker-compose logs apm-server  hey-apm elasticsearch
}
trap finish EXIT

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

docker-compose down -v
