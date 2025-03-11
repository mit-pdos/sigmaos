# syntax=docker/dockerfile:1-experimental

FROM alpine

RUN apk add --no-cache libseccomp \
  gfortran \
  gcompat \
  libffi-dev \
  musl-dev \
  curl \
  bash \
  gcc \
  libc-dev \
  parallel \
  libseccomp-static \
  make \
  python3-dev \
  openblas \
  lapack \
  tiff-dev \
  git

RUN echo 'will cite' | parallel --citation || true

WORKDIR /home/sigmaos
RUN mkdir -p bin/kernel && \
  mkdir -p bin/user

# Install specific version of OpenBLAS
RUN wget https://github.com/xianyi/OpenBLAS/releases/download/v0.3.23/OpenBLAS-0.3.23.tar.gz && \
  tar -xzf OpenBLAS-0.3.23.tar.gz && \
  rm OpenBLAS-0.3.23.tar.gz && \
  cd OpenBLAS-0.3.23 && \
  make USE_THREAD=1 INTERFACE64=1 DYNAMIC_ARCH=1 SYMBOLSUFFIX=64_

# Install Python
RUN git clone https://github.com/ivywu2003/cpython.git cpython3.11 && \
  cd cpython3.11 && \ 
  git checkout 3.11 && \
  git config pull.rebase false && \
  git pull && \
  ./configure --prefix=/home/sigmaos-local/bin/user --exec-prefix=/home/sigmaos-local/bin/user && \
  make -j

# Copy python user programs
COPY pyproc pyproc

# Install rust
RUN curl https://sh.rustup.rs -sSf | bash -s -- -y
RUN echo 'source $HOME/.cargo/env' >> $HOME/.bashrc
RUN source $HOME/.bashrc

# Copy rust trampoline
COPY rs rs 
ENV LIBSECCOMP_LINK_TYPE=static
ENV LIBSECCOMP_LIB_PATH="/usr/lib"

CMD [ "/bin/bash", "-l" ]
