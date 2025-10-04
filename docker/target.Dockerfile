# syntax=docker/dockerfile:1-experimental

FROM ubuntu:24.04 AS base

RUN apt update && \
  apt install -y \
  libseccomp-dev \
  strace \
  fuse \
  libspdlog-dev \
  libprotobuf-dev \
  valgrind \
  libc6-dbg \
  libabsl-dev \
  curl \
  golang \
  git \
  wget \
  gcc \
  g++ \
  pkg-config \
  parallel \
  time \
  cmake \
  ccache \
  libprotoc-dev \
  protobuf-compiler

# Install WasmEdge
RUN curl -sSf https://raw.githubusercontent.com/WasmEdge/WasmEdge/master/utils/install.sh | bash -s -- -v 0.14.1

# Set up WasmEdge environment variables
ENV WASMEDGE_DIR=/root/.wasmedge
ENV PATH="$WASMEDGE_DIR/bin:$PATH"
ENV LD_LIBRARY_PATH="$WASMEDGE_DIR/lib:$LD_LIBRARY_PATH"
ENV C_INCLUDE_PATH="$WASMEDGE_DIR/include:$C_INCLUDE_PATH"
ENV CPLUS_INCLUDE_PATH="$WASMEDGE_DIR/include:$CPLUS_INCLUDE_PATH"

# Copy WasmEdge libraries to /usr/lib so they're accessible in the uproc jail
RUN cp -a /root/.wasmedge/lib/* /usr/lib/

# Install wasmer go pkg
RUN mkdir t && \
  cd t && \
  go mod init tmod && \
  go get github.com/wasmerio/wasmer-go/wasmer@latest

WORKDIR /home/sigmaos
RUN mkdir bin && \
    mkdir all-realm-bin && \
    mkdir bin/user && \
    mkdir bin/kernel && \
    mkdir bin/linux

# ========== local user image ==========
FROM base AS sigmauser-local
RUN mkdir jail && \
    mkdir /tmp/spproxyd

# ========== remote user image ==========
FROM sigmauser-local AS sigmauser-remote
# Copy procd, the entrypoint for this container, to the user image.
COPY bin/kernel/procd bin/kernel/
# Copy spproxyd to the user image.
COPY bin/kernel/spproxyd bin/kernel/
## Copy rust trampoline to the user image.
COPY bin/kernel/uproc-trampoline /home/sigmaos/bin/kernel/

# ========== local kernel image ==========
FROM base AS sigmaos-local
WORKDIR /home/sigmaos
ENV kernelid kernel
ENV boot named
ENV dbip x.x.x.x
ENV mongoip x.x.x.x
ENV buildtag "local-build"
ENV dialproxy "false"
# Install docker-cli
RUN apt install -y docker.io
ENV reserveMcpu "0"
ENV netmode "host"
ENV sigmauser "NOT_SET"

# Make a directory for binaries shared between realms.
RUN mkdir -p /home/sigmaos/bin/user/common
CMD ["/bin/sh", "-c", "bin/linux/bootkernel ${kernelid} ${named} ${boot} ${dbip} ${mongoip} ${reserveMcpu} ${buildtag} ${dialproxy} ${netmode} ${sigmauser}"]

# ========== remote kernel image ==========
FROM sigmaos-local as sigmaos-remote
ENV buildtag "remote-build"
# Copy linux bins
COPY bin/linux /home/sigmaos/bin/linux/
# Copy kernel bins
COPY bin/kernel /home/sigmaos/bin/kernel/
# Copy script needed to set up network
COPY create-net.sh /home/sigmaos/bin/kernel/create-net.sh
# Copy named
RUN cp /home/sigmaos/bin/kernel/named /home/sigmaos/bin/user/common/named
CMD ["/bin/sh", "-c", "bin/linux/bootkernel ${kernelid} ${named} ${boot} ${dbip} ${mongoip} ${reserveMcpu} ${buildtag} ${dialproxy} ${netmode} ${sigmauser}"]
