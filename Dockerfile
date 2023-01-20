# syntax=docker/dockerfile:1

FROM golang AS base
RUN apt-get update
RUN apt-get install libseccomp-dev
RUN mkdir -p /home/sigmaos
WORKDIR /home/sigmaos
COPY go.mod ./
COPY go.sum ./
RUN go mod download

FROM base AS kernel
COPY . .
RUN ./make.sh --norace kernel
# XXX only necessary to make "cache" of binaries work in procd
RUN ./make.sh --norace user
RUN cp bin/kernel/named bin/user/named

CMD ["bin/linux/bootkernel", "bootkernelclnt/bootall.yml"]