#!/usr/bin/env bash
set -eo pipefail
set +x

function finish {
  set +e
  mkdir -p build
  {
    docker-compose version
    docker system info
    docker ps -a
    docker-compose logs apm-server validate-es-url hey-apm
    docker inspect --format "{{json .State }}" apm-server validate-es-url hey-apm
  } > build/environment.txt
  # Ensure all the sensitive details are obfuscated
  sed -i.bck -e "s#${ES_USER}#********#g" -e "s#${ES_PASS}#********#g" -e "s#${ES_URL}#********#g" build/environment.txt
  rm build/*.bck || true
  docker-compose down -v
  # To avoid running twice the same function and therefore override the environment.txt file.
  trap - INT QUIT TERM EXIT
  set -e
}

trap finish EXIT INT TERM

echo "Validate whether the ES_URL is reachable"
curl --user "${ES_USER}:${ES_PASS}" "${ES_URL}"

echo "Report ES stack health"
curl -s --user "${ES_USER}:${ES_PASS}" "${ES_URL}/_cluster/health"

echo "Launch test"
STACK_VERSION=${STACK_VERSION} \
ES_URL=${ES_URL} \
ES_USER=${ES_USER} \
ES_PASS=${ES_PASS} \
USER_ID="$(id -u):$(id -g)" docker-compose up \
  --no-color \
  --exit-code-from hey-apm \
  --build \
  --remove-orphans \
  --abort-on-container-exit \
  hey-apm
