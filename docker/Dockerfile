# from .. - docker build -f docker/Dockerfile .
FROM golang:1.9
RUN useradd hey

WORKDIR /go/src/github.com/elastic/hey-apm
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o hey-apm .

FROM scratch
MAINTAINER Elastic APM Team <docker@elastic.co>

# https://github.com/golang/go/blob/release-branch.go1.9/src/crypto/x509/root_linux.go
COPY docker/ca-certificates.crt /etc/ssl/certs/
COPY --from=0 /etc/passwd /etc/passwd
COPY --from=0 /go/src/github.com/elastic/hey-apm/hey-apm /hey-apm

USER hey
ENTRYPOINT ["/hey-apm"]
CMD ["-base-url", "http://apm-server:8200/"]
