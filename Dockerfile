# syntax=docker/dockerfile:1-experimental

FROM golang AS base
ARG parallel
ARG target=local
ARG tag
ENV SIGMATAG=$tag
# Install apt packages & clean up apt cache
RUN apt-get update && \
  apt-get --no-install-recommends --yes install libseccomp-dev && \
  apt clean && \
  apt autoclean && \
  apt autoremove && \
  rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/*
WORKDIR /home/sigmaos
COPY go.mod ./
COPY go.sum ./
RUN go mod download

COPY . .
# Build kernel and linux bins.
RUN --mount=type=cache,target=/root/.cache/go-build ./make.sh --norace --target $target $parallel kernel && \
  ./make.sh --norace --target $target $parallel user && \
  mkdir bin/user-common && \
  mv bin/user/* bin/user-common && \
  mv bin/user-common bin/user/common && \
  cp bin/kernel/named bin/user/common/named

# Copy bins to host
CMD ["sh", "-c", "cp -r --no-preserve=mode,ownership bin/user/* /tmp/bin"]
