# syntax=docker/dockerfile:1-experimental

FROM ubuntu:24.04

RUN apt update && \
  apt install -y \
  git \
  wget \
  gcc \
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
  protobuf-compiler

WORKDIR /home/sigmaos

CMD [ "/bin/bash", "-l" ]
