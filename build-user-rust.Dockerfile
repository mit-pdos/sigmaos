# syntax=docker/dockerfile:1-experimental

FROM alpine

RUN apk add --no-cache libseccomp \
  gcompat \
  musl-dev \
  curl \
  bash \
  gcc \
  libc-dev \
  libseccomp-static

WORKDIR /home/sigmaos
RUN mkdir -p bin/kernel && \
  mkdir -p bin/user

# Install rust
RUN curl https://sh.rustup.rs -sSf | bash -s -- -y
RUN echo 'source $HOME/.cargo/env' >> $HOME/.bashrc
RUN source $HOME/.bashrc

# Copy rust trampoline
COPY rs rs 
ENV LIBSECCOMP_LINK_TYPE=static
ENV LIBSECCOMP_LIB_PATH="/usr/lib"
RUN (cd rs/exec-uproc-rs && rm -rf target && $HOME/.cargo/bin/cargo build --release) && \
  cp rs/exec-uproc-rs/target/release/exec-uproc-rs bin/kernel && \
  (cd rs/spawn-latency && rm -rf target && $HOME/.cargo/bin/cargo build --release) && \
  cp rs/spawn-latency/target/release/spawn-latency bin/user

# When this container image is run, copy bins to host
CMD ["sh", "-c", "cp -r bin/user/* /tmp/bin/common/"]
