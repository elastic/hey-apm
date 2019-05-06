# from .. run: docker build -t hey-apm -f docker/Dockerfile .
FROM golang:1.12
RUN useradd hey

WORKDIR /build
COPY go.* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o hey-apm .

FROM scratch
MAINTAINER Elastic APM Team <docker@elastic.co>

# https://github.com/golang/go/blob/release-branch.go1.12/src/crypto/x509/root_linux.go
COPY --from=0 /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=0 /etc/passwd /etc/passwd
COPY --from=0 /build/hey-apm /hey-apm

USER hey
ENTRYPOINT ["/hey-apm"]
CMD ["-url", "http://apm-server:8200/"]
