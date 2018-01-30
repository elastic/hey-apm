FROM centos:7
RUN useradd hey

FROM scratch
MAINTAINER Elastic APM Team <docker@elastic.co>

# https://github.com/golang/go/blob/release-branch.go1.9/src/crypto/x509/root_linux.go
COPY ca-certificates.crt /etc/ssl/certs/

# CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o hey-apm .
COPY hey-apm /
COPY --from=0 /etc/passwd /etc/passwd

USER hey
ENTRYPOINT ["/hey-apm"]
CMD ["-base-url", "http://apm-server:8200/"]
