# syntax=docker/dockerfile:1-experimental

FROM golang AS base
ARG parallel
ARG target=local
ARG tag
ENV SIGMATAG=$tag
# Install apt packages & clean up apt cache
RUN apt-get update && \
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
RUN --mount=type=cache,target=/root/.cache/go-build ./make.sh --norace --target $target $parallel kernel && \
  ./make.sh --norace --target $target $parallel user && \
  mkdir bin/common && \
  mv bin/user/* bin/common && \
  mv bin/common bin/user/common && \
  cp bin/kernel/named bin/user/common/named
# Copy bins to host
CMD ["sh", "-c", "cp -r --no-preserve=mode,ownership bin/user/* /tmp/bin"]

# ========== user image ==========
FROM base AS sigmauser 
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
ENV overlays "false"
# Copy kernel bins
COPY --from=builder /home/sigmaos/bin/kernel /home/sigmaos/bin/kernel
# Copy linus bins
COPY --from=builder /home/sigmaos/bin/linux /home/sigmaos/bin/linux
CMD ["sh", "-c", "bin/linux/bootkernel ${kernelid} ${named} ${boot} ${dbip} ${overlays}"]

# ========== kernel image, including user binaries ==========
FROM sigmakernelclean AS sigmakernel
COPY --from=builder /home/sigmaos/bin/user /home/sigmaos/bin/user
