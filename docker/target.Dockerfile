# syntax=docker/dockerfile:1-experimental

FROM ubuntu:24.04 AS base

RUN apt update && \
  apt install -y \
  libseccomp-dev \
  strace \
  fuse3 \
  fuse-overlayfs \
  libspdlog-dev \
  libprotobuf-dev \
  valgrind \
  libc6-dbg \
  libabsl-dev \
  curl \
  golang \
  linux-tools-generic linux-tools-$(uname -r)

WORKDIR /home/sigmaos
RUN mkdir bin && \
    mkdir all-realm-bin && \
    mkdir bin/user && \
    mkdir bin/kernel && \
    mkdir bin/linux

COPY bin/kernel/lib/*.so /usr/lib/x86_64-linux-gnu

# ========== local user image ==========
FROM base AS sigmauser-local
RUN apt install -y \
  tini
ENTRYPOINT ["/usr/bin/tini", "--"]

RUN mkdir jail && \
    mkdir /tmp/sigmaos-proc-bundle-overlays && \
    mkdir /tmp/spproxyd && \
    mkdir /sigmaos-realm-bins-gvisor

RUN apt-get update && \
    apt-get install -y \
    apt-transport-https \
    ca-certificates \
    curl \
    gnupg \
    libevent-dev

# Install gVisor in user container
RUN curl -fsSL https://gvisor.dev/archive.key | gpg --dearmor -o /usr/share/keyrings/gvisor-archive-keyring.gpg
RUN echo "deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/gvisor-archive-keyring.gpg] https://storage.googleapis.com/gvisor/releases release main" | tee /etc/apt/sources.list.d/gvisor.list > /dev/null
RUN apt-get update && apt-get install -y runsc

# ========== remote user image ==========
FROM sigmauser-local AS sigmauser-remote
# Copy procd, the entrypoint for this container, to the user image.
COPY bin/kernel/procd bin/kernel/
# Copy spproxyd to the user image.
COPY bin/kernel/spproxyd bin/kernel/
## Copy rust trampoline to the user image.
COPY bin/kernel/uproc-trampoline /home/sigmaos/bin/kernel/
## Copy python interpreters to the user image.
COPY bin/kernel/cpython3.11 /home/sigmaos/bin/kernel/cpython3.11
## Copy pyproc to the user image. In the future, this should be fetched from s3.
COPY bin/kernel/pyproc /home/sigmaos/bin/kernel/pyproc

# ========== local kernel image ==========
FROM base AS sigmaos-local
WORKDIR /home/sigmaos
ENV kernelid kernel
ENV boot named
ENV dbip x.x.x.x
ENV mongoip x.x.x.x
ENV buildtag "local-build"
ENV dialproxy "false"
ENV gvisor "false"
# Install docker-cli
RUN apt update && apt install -y docker.io
ENV reserveMcpu "0"
ENV netmode "host"
ENV sigmauser "NOT_SET"

# Make a directory for binaries shared between realms.
RUN mkdir -p /home/sigmaos/bin/user/common
CMD ["/bin/sh", "-c", "bin/linux/bootkernel ${kernelid} ${named} ${boot} ${dbip} ${mongoip} ${reserveMcpu} ${buildtag} ${dialproxy} ${netmode} ${sigmauser} ${gvisor}"]

# ========== remote kernel image ==========
FROM sigmaos-local AS sigmaos-remote
ENV buildtag "remote-build"
# Copy linux bins
COPY bin/linux /home/sigmaos/bin/linux/
# Copy kernel bins
COPY bin/kernel /home/sigmaos/bin/kernel/
# Copy script needed to set up network
COPY create-net.sh /home/sigmaos/bin/kernel/create-net.sh
# Copy named
RUN cp /home/sigmaos/bin/kernel/named /home/sigmaos/bin/user/common/named
CMD ["/bin/sh", "-c", "bin/linux/bootkernel ${kernelid} ${named} ${boot} ${dbip} ${mongoip} ${reserveMcpu} ${buildtag} ${dialproxy} ${netmode} ${sigmauser} ${gvisor}"]
