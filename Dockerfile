# syntax=docker/dockerfile:1-experimental

FROM golang AS base
ARG parallel
ARG target=local
# Install apt packages & clean up apt cache
RUN apt-get update && \
  apt-get --no-install-recommends --yes install iputils-ping libseccomp-dev && \
  apt clean && \
  apt autoclean && \
  apt autoremove && \
  rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/*
WORKDIR /home/sigmaos
COPY go.mod ./
COPY go.sum ./
RUN go mod download

FROM base AS kernel
COPY . .
RUN --mount=type=cache,target=/root/.cache/go-build ./make.sh --norace --target $target $parallel kernel
# XXX only necessary to make "cache" of binaries work in procd
RUN --mount=type=cache,target=/root/.cache/go-build ./make.sh --norace --target $target $parallel user
RUN cp bin/kernel/named bin/user/named
