FROM golang:1.26-trixie AS builder

WORKDIR $GOPATH/src/github.com/matthope/dockerhub-ratelimit-exporter

COPY go.mod go.sum main.go ./

RUN go mod download -x

ARG TARGETOS
ARG TARGETARCH

RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build -ldflags "-s -w" -o /out/dockerhub-ratelimit-exporter ./

FROM scratch

COPY --from=builder /etc/ssl/certs/ /etc/ssl/certs/
COPY --from=builder /out/dockerhub-ratelimit-exporter /bin/dockerhub-ratelimit-exporter


ENTRYPOINT ["/bin/dockerhub-ratelimit-exporter"]
