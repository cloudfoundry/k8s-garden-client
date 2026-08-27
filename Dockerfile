ARG GO_VERSION=1.27

FROM --platform=$BUILDPLATFORM golang:${GO_VERSION} AS builder

ARG TARGETARCH
ARG GO_VERSION

WORKDIR /src

COPY . .
RUN go mod download

RUN GOFLAGS="-gcflags=all=-lang=go${GO_VERSION}" GOOS=linux GOARCH=${TARGETARCH} go build -ldflags "-w -s" -o bin/rep ./cmd/rep
RUN GOFLAGS="-gcflags=all=-lang=go${GO_VERSION}" GOOS=linux GOARCH=${TARGETARCH} go build -ldflags "-w -s" -o bin/watcher ./cmd/watch

FROM ubuntu:26.04
ARG TARGETARCH
RUN apt-get update && apt-get install -y \
    ca-certificates \
    tzdata \
    && \
    update-ca-certificates

COPY --from=builder /src/bin/rep /bin/rep
COPY --from=builder /src/bin/watcher /bin/watcher

EXPOSE 8080 443

ENTRYPOINT [ "/bin/rep" ]
