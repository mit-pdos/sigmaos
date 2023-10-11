# syntax=docker/dockerfile:1-experimental

FROM alpine

RUN apk add --no-cache libseccomp \
  gcompat \
  libpthread-stubs \
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
RUN (cd rs/exec-uproc-rs && $HOME/.cargo/bin/cargo build) && \
  cp rs/exec-uproc-rs/target/debug/exec-uproc-rs bin/kernel && \
  (cd rs/spawn-latency && $HOME/.cargo/bin/cargo build) && \
  cp rs/spawn-latency/target/debug/spawn-latency bin/user

RUN touch /home/sigmaos/bin/user/test-rust-bin

# When this container image is run, copy bins to host
CMD ["sh", "-c", "cp -r bin/user/* /tmp/bin/common/"]
