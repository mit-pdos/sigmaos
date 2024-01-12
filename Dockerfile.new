# syntax=docker/dockerfile:1-experimental

FROM alpine AS base

# Install some apt packages for debugging.
#RUN \
#  apt-get update && \
#  apt-get --no-install-recommends --yes install iputils-ping && \
#  apt-get --no-install-recommends --yes install iproute2 && \
#  apt-get --no-install-recommends --yes install netcat-traditional && \
#  apt clean && \
#  apt autoclean && \
#  apt autoremove && \
#  rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/*

RUN apk add --no-cache libseccomp gcompat musl-dev strace

WORKDIR /home/sigmaos
RUN mkdir bin && \
    mkdir all-realm-bin && \
    mkdir bin/user && \
    mkdir bin/kernel && \
    mkdir bin/linux

# ========== user image ==========
FROM base AS sigmauser

RUN mkdir jail && \
    mkdir /tmp/sigmaclntd

# XXX needed still?
# Copy mr yaml files.
#COPY mr mr

# Copy uprocd, the entrypoint for this container, to the user image.
COPY bin/kernel/uprocd bin/kernel/
# Copy sigmaclntd to the user image.
COPY bin/kernel/sigmaclntd bin/kernel/
## Copy rust trampoline to the user image.
COPY bin/kernel/exec-uproc-rs /home/sigmaos/bin/kernel/

# ========== kernel image, omitting user binaries ==========
FROM base AS sigmaos
WORKDIR /home/sigmaos
ENV kernelid kernel
ENV boot named
ENV dbip x.x.x.x
ENV mongoip x.x.x.x
ENV overlays "false"
ENV gvisor "false"
# Install docker-cli
RUN apk add --update docker openrc
ENV reserveMcpu "0"

# Hack to invalidate build cache
#ARG kill_build_cache
#
#RUN echo "Invalidating build cache kernel $kill_build_cache"

# Copy kernel bins
COPY bin/kernel /home/sigmaos/bin/kernel/
COPY create-net.sh /home/sigmaos/bin/kernel/create-net.sh
# Copy named bin
RUN mkdir -p /home/sigmaos/bin/user/common && \
  cp /home/sigmaos/bin/kernel/named /home/sigmaos/bin/user/common/named
# Needed?
# Copy linux bins
COPY bin/linux /home/sigmaos/bin/linux/
CMD ["/bin/sh", "-c", "bin/linux/bootkernel ${kernelid} ${named} ${boot} ${dbip} ${mongoip} ${overlays} ${reserveMcpu} ${gvisor}"]

# ========== kernel image, including user binaries ==========
FROM sigmaos AS sigmaos-with-userbin
COPY bin/user/* /home/sigmaos/bin/user/common

CMD ["/bin/sh", "-c", "bin/linux/bootkernel ${kernelid} ${named} ${boot} ${dbip} ${mongoip} ${overlays} ${reserveMcpu} ${gvisor}"]
