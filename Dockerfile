# syntax=docker/dockerfile:1-experimental

FROM golang AS base
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

# Install docker ce-cli
RUN apt-get update
RUN apt-get --yes install ca-certificates curl gnupg lsb-release
RUN mkdir -m 0755 -p /etc/apt/keyrings
RUN curl -fsSL https://download.docker.com/linux/debian/gpg | gpg --dearmor -o /etc/apt/keyrings/docker.gpg
RUN echo \
  "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/debian \
  $(lsb_release -cs) stable" | tee /etc/apt/sources.list.d/docker.list > /dev/null
RUN apt-get --yes update && apt-get --yes install docker-ce-cli

# Install apt packages & clean up apt cache
RUN \
  apt-get --no-install-recommends --yes install libseccomp-dev && \
  apt-get --no-install-recommends --yes install iputils-ping && \
  apt-get --no-install-recommends --yes install iproute2 && \
  apt-get --no-install-recommends --yes install netcat-traditional && \
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

# ========== binary builder image ==========
FROM base AS builder
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

# ========== user image ==========
FROM base AS sigmauser 
RUN mkdir jail
# Copy mr yaml files.
COPY mr mr
# Copy uprocd, the entrypoint for this container, to the user image.
COPY --from=builder /home/sigmaos/bin/kernel/uprocd /home/sigmaos/bin/kernel
# Copy exec-uproc, the trampoline program, to the user image, 
COPY --from=builder /home/sigmaos/bin/user/common/exec-uproc /home/sigmaos/bin/kernel

# ========== kernel image, omitting user binaries ==========
FROM base AS sigmakernelclean
WORKDIR /home/sigmaos
ENV kernelid kernel
ENV named :1111
ENV boot named
ENV dbip x.x.x.x
ENV jaegerip x.x.x.x
ENV overlays "false"
# Copy kernel bins
COPY --from=builder /home/sigmaos/bin/kernel /home/sigmaos/bin/kernel
COPY --from=builder /home/sigmaos/create-net.sh /home/sigmaos/bin/kernel/create-net.sh
# Copy linus bins
COPY --from=builder /home/sigmaos/bin/linux /home/sigmaos/bin/linux
CMD ["sh", "-c", "bin/linux/bootkernel ${kernelid} ${named} ${boot} ${dbip} ${jaegerip} ${overlays}"]

# ========== kernel image, including user binaries ==========
FROM sigmakernelclean AS sigmakernel
COPY --from=builder /home/sigmaos/bin/user /home/sigmaos/bin/user
