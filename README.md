Basic load generation for apm-server built on [hey](https://github.com/rakyll/hey).

# Install

```
# create vendor/
go get github.com/golang/dep/cmd/dep
dep ensure -v
```

# Docker build

```
docker build -f docker/Dockerfile .
```

# Known issues / Todo

* The race detector is unhappy - https://github.com/rakyll/hey/issues/85
* Response latency distribution
