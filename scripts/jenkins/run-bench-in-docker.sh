#!/usr/bin/env bash
set -xeo pipefail

USER_ID="$(id -u):$(id -g)"
STACK_VERSION=${STACK_VERSION}
ES_URL=${ES_URL}
ES_USER=${ES_USER}
ES_PASS=${ES_PASS}
export USER_ID STACK_VERSION ES_URL ES_USER ES_PASS

function finish {
  set +e
  mkdir -p build
  {
    docker-compose version
    docker system info
    docker ps -a
    docker-compose logs apm-server elasticsearch validate-es-url hey-apm
    docker inspect --format "{{json .State }}" apm-server elasticsearch validate-es-url hey-apm
  } > build/environment.txt
  docker-compose down -v
  # To avoid running twice the same function and therefore override the environment.txt file.
  trap - INT QUIT TERM EXIT
  set -e
}

trap finish EXIT INT TERM

## Validate whether the ES_URL is reachable
curl --user "${ES_USER}:${ES_PASS}" "${ES_URL}"

docker-compose up \
  --no-color \
  --exit-code-from hey-apm \
  --remove-orphans \
  --abort-on-container-exit \
  hey-apm
