FROM golang:1.15-buster AS builder

WORKDIR $GOPATH/src/github.com/matthope/dockerhub-ratelimit-exporter

COPY go.mod go.sum main.go .

RUN go mod download -x

RUN go build -ldflags "-linkmode external -extldflags -static -s -w" ./

FROM scratch

COPY --from=builder /etc/ssl/certs/ /etc/ssl/certs/
COPY --from=builder /go/src/github.com/matthope/dockerhub-ratelimit-exporter/dockerhub-ratelimit-exporter /bin/dockerhub-ratelimit-exporter


ENTRYPOINT ["/bin/dockerhub-ratelimit-exporter"]
