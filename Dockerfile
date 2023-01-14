# syntax=docker/dockerfile:1

FROM golang
RUN mkdir -p /home/sigmaos
RUN mkdir -p /home/sigmaos/rootrealm
RUN mkdir -p /home/sigmaos/rootrealm/sys
RUN mkdir -p /home/sigmaos/rootrealm/dev
RUN mkdir -p /home/sigmaos/rootrealm/usr
RUN mkdir -p /home/sigmaos/rootrealm/lib
RUN mkdir -p /home/sigmaos/rootrealm/lib64
RUN mkdir -p /home/sigmaos/rootrealm/etc
RUN mkdir -p /home/sigmaos/rootrealm/bin
RUN mkdir -p /home/sigmaos/rootrealm/proc
COPY bin/user/ /home/sigmaos/rootrealm/bin/user
WORKDIR /home/sigmaos
COPY go.mod ./
COPY go.sum ./
RUN go mod download
COPY . .
RUN ./make.sh --norace
CMD ["bin/linux/bootkernel", "bootkernelclnt/bootall.yml"]