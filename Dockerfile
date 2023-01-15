# syntax=docker/dockerfile:1

FROM golang AS base
RUN mkdir -p /home/sigmaos
WORKDIR /home/sigmaos
COPY go.mod ./
COPY go.sum ./
RUN go mod download

FROM base AS kernel
COPY . .
RUN ./make.sh --norace kernel

CMD ["bin/linux/bootkernel", "bootkernelclnt/bootall.yml"]