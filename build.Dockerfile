# syntax=docker/dockerfile:1-experimental

FROM archlinux

RUN yes | pacman -Syu
RUN yes | pacman -Sy git libseccomp wget gcc pkg-config parallel

# Download an initial version of Go
RUN wget "https://go.dev/dl/go1.20.4.linux-amd64.tar.gz" && \
  tar -C /usr/local -xzf go1.20.4.linux-amd64.tar.gz

# Set the PATH to include the new Go install.
ENV PATH="${PATH}:/usr/local/go/bin"

# Install custom version of go with larger minimum stack size.
RUN git clone https://github.com/ArielSzekely/go.git go-custom && \
  cd go-custom && \
  git checkout bigstack && \
  git config pull.rebase false && \
  git pull && \
  cd src && \
  ./make.bash

# Install some apt packages for debugging.
#RUN \
#  apt-get update && \
#  apt-get --no-install-recommends --yes install iputils-ping && \
#  apt-get --no-install-recommends --yes install iproute2 && \
#  apt-get --no-install-recommends --yes install netcat-traditional && \
#  apt clean && \
#  apt autoclean && \
#  apt autoremove && \
#  rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/*

# Install rust
RUN curl https://sh.rustup.rs -sSf | bash -s -- -y
ENV PATH="${PATH}:$HOME/.cargo/bin"
RUN $HOME/.cargo/bin/rustup target add x86_64-unknown-linux-musl

WORKDIR /home/sigmaos

RUN mkdir bin && \
    mkdir bin/user && \
    mkdir bin/kernel && \
    mkdir bin/linux
# Copy some yaml files to the base image.
COPY seccomp seccomp

# Download go modules
COPY go.mod ./
COPY go.sum ./
RUN go mod download

ARG parallel
ARG target
ARG userbin
ARG tag

# Set env after downlaoding and installing the custom Go version, so we don't
# rebuild Go evey time we change tags.
ENV SIGMATAG=$tag

# Copy source
COPY . .
# Build all binaries.
RUN --mount=type=cache,target=/root/.cache/go-build ./make.sh --norace --gopath /go-custom/bin/go --target $target $parallel kernel && \
  ./make.sh --norace --gopath /go-custom/bin/go --userbin $userbin --target $target $parallel user && \
  mkdir bin/common && \
  mv exec-uproc-rs/target/debug/exec-uproc-rs bin/common && \
  mv bin/user/* bin/common && \
  mv bin/common bin/user/common && \
  cp bin/kernel/named bin/user/common/named
# Copy bins to host
CMD ["sh", "-c", "cp -r --no-preserve=mode,ownership bin/user/* /tmp/bin"]
