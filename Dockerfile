ARG GO_VERSION=1.27

FROM gcc:15 AS gccbuild

ARG TAR_VERSION=1.35

WORKDIR /src

RUN mkdir -p ./tar && curl -L http://ftp.gnu.org/gnu/tar/tar-${TAR_VERSION}.tar.xz | tar -xJ -C ./tar

ENV LDFLAGS=-static
ENV FORCE_UNSAFE_CONFIGURE=1
ENV CC="musl-gcc -static"

RUN apt update && apt install musl musl-dev musl-tools -y && \
    cd ./tar/tar-${TAR_VERSION} && ./configure && make && mv src/tar /src/tar/tar

FROM --platform=$BUILDPLATFORM golang:${GO_VERSION} AS builder

ARG TARGETARCH
ARG GO_VERSION

WORKDIR /src

COPY . .
RUN go mod download

RUN CGO_ENABLED=0 GOFLAGS="-gcflags=all=-lang=go${GO_VERSION}" GOOS=linux GOARCH=${TARGETARCH} go build -ldflags "-w -s" -o bin/rep ./cmd/rep
RUN CGO_ENABLED=0 GOFLAGS="-gcflags=all=-lang=go${GO_VERSION}" GOOS=linux GOARCH=${TARGETARCH} go build -ldflags "-w -s" -o bin/watcher ./cmd/watch
RUN CGO_ENABLED=0 GOFLAGS="-gcflags=all=-lang=go${GO_VERSION}" GOOS=linux GOARCH=${TARGETARCH} go build -ldflags "-w -s" -o bin/untar ./cmd/untar

FROM ubuntu:26.04
ARG TARGETARCH
RUN apt-get update && apt-get install -y \
    ca-certificates \
    tzdata \
    && \
    update-ca-certificates

COPY --from=builder /src/bin/rep /bin/rep
COPY --from=builder /src/bin/watcher /bin/watcher
COPY --from=builder /src/bin/untar /bin/untar
COPY --from=gccbuild /src/tar/tar /bin/tar

EXPOSE 8080 443

ENTRYPOINT [ "/bin/rep" ]
