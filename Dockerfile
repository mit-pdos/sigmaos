# syntax=docker/dockerfile:1

FROM golang
RUN mkdir -p /home/sigmaos
WORKDIR /home/sigmaos
COPY go.mod ./
COPY go.sum ./
RUN go mod download
COPY . .
RUN ./make.sh --norace
