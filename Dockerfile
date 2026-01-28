FROM gcc:latest AS gccbuild

ARG TAR_VERSION=1.35

WORKDIR /src

ADD https://github.com/cloudfoundry/guardian.git#main:rundmc/nstar ./nstar
# dockerfile-utils: ignore
ADD --unpack=true http://ftp.gnu.org/gnu/tar/tar-${TAR_VERSION}.tar.xz ./tar

ENV LDFLAGS=-static
ENV FORCE_UNSAFE_CONFIGURE=1

RUN apt update && apt install musl musl-dev musl-tools -y && \
    sed -i '/#include <unistd.h>/a #include <fcntl.h>' nstar/nstar.c && \
    make -C ./nstar nstar && \
    cd ./tar/tar-${TAR_VERSION} && CC="musl-gcc -static" ./configure && CC="musl-gcc -static" make && mv src/tar /src/tar/tar

FROM --platform=$BUILDPLATFORM golang:1 AS build
ARG TARGETOS TARGETARCH
WORKDIR /src

RUN --mount=type=cache,target=/go/pkg/mod/ \
    --mount=type=bind,source=go.sum,target=go.sum \
    --mount=type=bind,source=go.mod,target=go.mod \
    go mod download -x

RUN --mount=type=cache,target=/go/pkg/mod/ \
    --mount=type=bind,target=. \
    GOFLAGS="-gcflags=all=-lang=$(go version | awk '{print $3}' | sed 's/\.[0-9]*$//')" CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH} go build -ldflags "-w -s" -trimpath -o /bin/server ./cmd/rep

RUN --mount=type=cache,target=/go/pkg/mod/ \
    --mount=type=bind,target=. \
    GOFLAGS="-gcflags=all=-lang=$(go version | awk '{print $3}' | sed 's/\.[0-9]*$//')" CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH} go build -ldflags "-w -s" -trimpath -o /bin/watcher ./cmd/watch

FROM ubuntu:24.04
ARG TARGETARCH
RUN apt-get update && apt-get install -y \
    ca-certificates \
    tzdata \
    && \
    update-ca-certificates

COPY --from=build /bin/server /bin/
COPY --from=build /bin/watcher /bin/
COPY --from=gccbuild /src/nstar/nstar /bin/
COPY --from=gccbuild /src/tar/tar /bin/

EXPOSE 8080 443

ENTRYPOINT [ "/bin/server" ]
