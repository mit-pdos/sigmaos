# syntax=docker/dockerfile:1-experimental

FROM alpine AS base

RUN apk add --no-cache libseccomp gcompat musl-dev strace fuse

WORKDIR /home/sigmaos
RUN mkdir bin

COPY bin/user/hotel-wwwd-v1.0 bin/
COPY bin/user/imgresize-v1.0 bin/
