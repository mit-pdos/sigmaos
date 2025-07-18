# syntax=docker/dockerfile:1-experimental

FROM alpine

RUN apk add --no-cache libseccomp \
  gcompat \
  musl-dev \
  curl \
  bash \
  gcc \
  libc-dev \
  parallel \
  libseccomp-static

RUN echo 'will cite' | parallel --citation || true

WORKDIR /home/sigmaos
RUN mkdir -p bin/kernel && \
  mkdir -p bin/user

# Install rust
RUN curl https://sh.rustup.rs -sSf | bash -s -- -y
RUN echo 'source $HOME/.cargo/env' >> $HOME/.bashrc
RUN source $HOME/.bashrc
ENV PATH="/root/.cargo/bin:${PATH}"
RUN rustup update
RUN cargo install wasm-pack

# Copy rust trampoline
COPY rs rs 
ENV LIBSECCOMP_LINK_TYPE=static
ENV LIBSECCOMP_LIB_PATH="/usr/lib"

CMD [ "/bin/bash", "-l" ]
