# syntax=docker/dockerfile:1-experimental

FROM golang
ARG parallel
ARG target=local
ARG tag
ENV SIGMATAG=$tag

# Install custom version of go with larger minimum stack size.
RUN git clone https://github.com/ArielSzekely/go.git go-custom && \
  cd go-custom && \
  git checkout bigstack && \
  git config pull.rebase false && \
  git pull && \
  cd src && \
  ./all.bash

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

# Install apt packages & clean up apt cache
RUN \
  apt-get update && \
  apt-get --no-install-recommends --yes install libseccomp-dev && \
  apt clean && \
  apt autoclean && \
  apt autoremove && \
  rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/*

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
# Copy source
COPY . .
# Build all binaries.
RUN --mount=type=cache,target=/root/.cache/go-build ./make.sh --norace --gopath /go/go-custom/bin/go --target $target $parallel kernel && \
  ./make.sh --norace --gopath /go/go-custom/bin/go --target $target $parallel user && \
  mkdir bin/common && \
  mv bin/user/* bin/common && \
  mv bin/common bin/user/common && \
  cp bin/kernel/named bin/user/common/named
# Copy bins to host
CMD ["sh", "-c", "cp -r --no-preserve=mode,ownership bin/user/* /tmp/bin"]
