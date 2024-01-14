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

# ========== local user image ==========
FROM base AS sigmauser-local
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

# ========== remote user image ==========
FROM sigmauser-local AS sigmauser-remote

# ========== local kernel image ==========
FROM base AS sigmaos-local
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

# Copy script needed to set up network
COPY create-net.sh /home/sigmaos/bin/kernel/create-net.sh
# Make a directory for binaries shared between realms.
RUN mkdir -p /home/sigmaos/bin/user/common
CMD ["/bin/sh", "-c", "bin/linux/bootkernel ${kernelid} ${named} ${boot} ${dbip} ${mongoip} ${overlays} ${reserveMcpu} ${gvisor}"]

# ========== remote kernel image ==========
FROM sigmaos-local as sigmaos-remote
# Copy linux bins
COPY bin/linux /home/sigmaos/bin/linux/
# Copy kernel bins
COPY bin/kernel /home/sigmaos/bin/kernel/
# Copy user bins
COPY bin/user/* /home/sigmaos/bin/user/common
# Copy named
RUN cp /home/sigmaos/bin/kernel/named /home/sigmaos/bin/user/common/named
CMD ["/bin/sh", "-c", "bin/linux/bootkernel ${kernelid} ${named} ${boot} ${dbip} ${mongoip} ${overlays} ${reserveMcpu} ${gvisor}"]
