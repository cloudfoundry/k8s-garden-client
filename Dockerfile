FROM gcc:latest AS gccbuild

ARG TAR_VERSION=1.35

WORKDIR /src

ADD https://github.com/cloudfoundry/guardian.git#main:rundmc/nstar ./nstar
RUN mkdir -p ./tar && curl -L http://ftp.gnu.org/gnu/tar/tar-${TAR_VERSION}.tar.xz | tar -xJ -C ./tar

ENV LDFLAGS=-static
ENV FORCE_UNSAFE_CONFIGURE=1

RUN apt update && apt install musl musl-dev musl-tools -y && \
    sed -i '/#include <unistd.h>/a #include <fcntl.h>' nstar/nstar.c && \
    make -C ./nstar nstar && \
    cd ./tar/tar-${TAR_VERSION} && CC="musl-gcc -static" ./configure && CC="musl-gcc -static" make && mv src/tar /src/tar/tar

FROM ubuntu:24.04
ARG TARGETARCH
RUN apt-get update && apt-get install -y \
    ca-certificates \
    tzdata \
    && \
    update-ca-certificates

COPY bin/rep /bin/rep
COPY bin/watcher /bin/watcher
COPY --from=gccbuild /src/nstar/nstar /bin/
COPY --from=gccbuild /src/tar/tar /bin/

EXPOSE 8080 443

ENTRYPOINT [ "/bin/rep" ]
