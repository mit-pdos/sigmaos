# syntax=docker/dockerfile:1-experimental

FROM golang AS base
ARG parallel
ARG target=local
RUN apt-get update
RUN apt-get install libseccomp-dev
RUN apt-get --yes install iputils-ping
RUN apt-get --yes install iproute2
RUN mkdir -p /home/sigmaos
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
