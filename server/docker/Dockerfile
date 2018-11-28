FROM golang:1.11.2
RUN apt-get update && \
    apt-get install -y --no-install-recommends python-pip virtualenv build-essential
RUN mkdir -p ${GOPATH}/src/github.com/elastic/
WORKDIR ${GOPATH}/src/github.com/elastic/
ARG nocache
RUN git clone https://github.com/elastic/apm-server.git && \
  cd ${GOPATH}/src/github.com/elastic/apm-server && \
  git remote add axw https://github.com/axw/apm-server.git && \
  git remote add graphaelli https://github.com/graphaelli/apm-server.git && \
  git remote add jalvz https://github.com/jalvz/apm-server.git && \
  git remote add roncohen https://github.com/roncohen/apm-server.git && \
  git remote add simitt https://github.com/simitt/apm-server.git && \
  go get -v -u github.com/magefile/mage && \
  (cd $GOPATH/src/github.com/magefile/mage && go run bootstrap.go)
WORKDIR ${GOPATH}/src/github.com/elastic/apm-server
ENV PATH="${GOPATH}/bin:${PATH}"
ARG fetch_cmd
ARG checkout_cmd
ARG make_update_cmd
ARG make_cmd
RUN ${fetch_cmd} && \
  ${checkout_cmd} && \
  ${make_update_cmd} && \
  ${make_cmd}
EXPOSE 8200
