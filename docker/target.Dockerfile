# syntax=docker/dockerfile:1-experimental

FROM ubuntu:22.04 AS base

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
  protobuf-compiler \
  build-essential \
  libssl-dev \
  python3 \
  python3-pip \
  clang \
  lld

# Install LLVM 13
RUN apt install -y \
  llvm-13 \
  llvm-13-dev \
  llvm-13-runtime \
  clang-13 \
  libclang-13-dev \
  liblld-13-dev

# Set up LLVM environment variables
ENV LLVM_SYS_130_PREFIX=/usr/lib/llvm-13
ENV LLVM_DIR=/usr/lib/llvm-13
ENV PATH=/usr/lib/llvm-13/bin:$PATH

# Install Rust to build Wasmer
RUN curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh -s -- -y
ENV PATH="/root/.cargo/bin:$PATH"

# Clone and build Wasmer 2.3.0 with LLVM support
WORKDIR /tmp
RUN git clone https://github.com/wasmerio/wasmer.git && \
  cd wasmer && \
  git checkout 2.3.0 && \
  cargo build --release --features llvm,singlepass,cranelift

# Build Wasmer C API with LLVM support
WORKDIR /tmp/wasmer/lib/c-api
RUN cargo build --release --features llvm,singlepass,cranelift 

WORKDIR /tmp/wasmer
RUN make package-capi

# Install Wasmer C API system-wide
RUN cd package && \
  cp lib/* /usr/local/lib/ && \
  cp include/* /usr/local/include/ && \
  ldconfig

# Set up Wasmer environment variables
ENV LD_LIBRARY_PATH=/usr/local/lib:$LD_LIBRARY_PATH
ENV CGO_LDFLAGS="-L/usr/local/lib -lwasmer"
ENV CGO_CFLAGS="-I/usr/local/include"

# Clean up temporary build files 
RUN rm -rf /tmp/wasmer 

# Install wasmer go pkg
RUN mkdir t && \
  cd t && \
  go mod init tmod && \
  go get github.com/wasmerio/wasmer-go/wasmer@latest && \
  cp /root/go/pkg/mod/github.com/wasmerio/wasmer-go\@v1.0.4/wasmer/packaged/lib/linux-amd64/libwasmer.so /lib/

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
