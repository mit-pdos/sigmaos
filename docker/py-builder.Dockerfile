# syntax=docker/dockerfile:1-experimental

FROM ubuntu:24.04

RUN apt update && \
  apt install -y \
    git \
    wget \
    zip \
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
    libffi-dev \
    libssl-dev \
    uuid-dev \
    lzma-dev \
    liblzma-dev \
    libbz2-dev \
    libprotoc-dev \
    protobuf-compiler

# Install Python
RUN wget https://github.com/python/cpython/archive/refs/tags/v3.11.13.tar.gz -O /cpython.tar.gz && \
    tar -xzf /cpython.tar.gz -C / && \
    rm /cpython.tar.gz && \
    mv /cpython-3.11.13 /cpython3.11 && \
    cd /cpython3.11 && \
    ./configure \
      --prefix=/tmp/python/python && \
    make -j

# Precompile all Python libraries
RUN /cpython3.11/python -m compileall -j8 -f /cpython3.11/Lib || true

# Set up builder user
ARG USER_ID=1000
ARG GROUP_ID=1000

RUN groupadd -g ${GROUP_ID} builder && \
    useradd -m -u ${USER_ID} -g ${GROUP_ID} -s /bin/bash builder

USER builder

WORKDIR /home/sigmaos

CMD [ "/bin/bash", "-l" ]
