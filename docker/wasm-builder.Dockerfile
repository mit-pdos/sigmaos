# syntax=docker/dockerfile:1-experimental

FROM ubuntu:24.04

RUN apt update && \
  apt install -y \
  git \
  wget \
  gcc \
  g++ \
  pkg-config \
  parallel \
  time \
  cmake \
  ccache \
  libprotobuf-dev \
  libseccomp-dev \
  libspdlog-dev \
  libabsl-dev \
  libprotoc-dev \
  protobuf-compiler \
  curl

# Install WasmEdge
RUN curl -sSf https://raw.githubusercontent.com/WasmEdge/WasmEdge/master/utils/install.sh | bash -s -- -v 0.14.1

# Set up WasmEdge environment variables (for headers when compiling .wasm modules)
ENV WASMEDGE_DIR=/root/.wasmedge
ENV PATH="$WASMEDGE_DIR/bin:$PATH"
ENV C_INCLUDE_PATH="$WASMEDGE_DIR/include:$C_INCLUDE_PATH"
ENV CPLUS_INCLUDE_PATH="$WASMEDGE_DIR/include:$CPLUS_INCLUDE_PATH"

# Install wasi-sdk
RUN wget https://github.com/WebAssembly/wasi-sdk/releases/download/wasi-sdk-24/wasi-sdk-24.0-x86_64-linux.tar.gz && \
  tar xvf wasi-sdk-24.0-x86_64-linux.tar.gz && \
  mv wasi-sdk-24.0-x86_64-linux /opt/wasi-sdk && \
  rm wasi-sdk-24.0-x86_64-linux.tar.gz

WORKDIR /home/sigmaos

CMD [ "/bin/bash", "-l" ]
