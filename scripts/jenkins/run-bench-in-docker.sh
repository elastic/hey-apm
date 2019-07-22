#!/usr/bin/env bash
set -eo pipefail

STACK_VERSION=${STACK_VERSION} USER_ID="$(id -u):$(id -g)" docker-compose \
  --no-ansi \
  --log-level ERROR \
  up \
  --exit-code-from hey-apm \
  --build \
  --remove-orphans \
  --abort-on-container-exit \
  hey-apm

docker-compose \
  --no-ansi \
  --log-level ERROR \
  down -v
