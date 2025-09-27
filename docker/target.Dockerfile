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
  golang

# Install wasmer go pkg
RUN mkdir t && \
  cd t && \
  go mod init tmod && \
  go get github.com/wasmerio/wasmer-go/wasmer@latest

# Install libwasmer
RUN mkdir wasmer && \
  curl https://get.wasmer.io -sSfL | WASMER_DIR=wasmer sh && \
  cp wasmer/lib/* /lib/

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
