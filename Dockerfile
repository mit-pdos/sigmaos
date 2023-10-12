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

RUN apk add --no-cache libseccomp gcompat libpthread-stubs musl-dev strace

WORKDIR /home/sigmaos
RUN mkdir bin && \
    mkdir bin/user && \
    mkdir bin/kernel && \
    mkdir bin/linux

ARG tag

ENV SIGMATAG=$tag

# ========== user image ==========
FROM base AS sigmauser

RUN mkdir jail
# Copy mr yaml files.
COPY mr mr

# Hack to invalidate build cache
#ARG kill_build_cache
#
#RUN echo "Invalidating build cache user $kill_build_cache"

# Copy uprocd, the entrypoint for this container, to the user image.
COPY --from=sigma-build-kernel /home/sigmaos/bin/kernel/uprocd /home/sigmaos/bin/kernel
# Copy rust trampoline to the user image.
COPY --from=sigma-build-user-rust /home/sigmaos/bin/kernel/exec-uproc-rs /home/sigmaos/bin/kernel/exec-uproc-rs

# ========== kernel image, omitting user binaries ==========
FROM base AS sigmaos
WORKDIR /home/sigmaos
ENV kernelid kernel
ENV boot named
ENV dbip x.x.x.x
ENV mongoip x.x.x.x
ENV overlays "false"
# Install docker-cli
RUN apk add --update docker openrc
ENV reserveMcpu "0"

# Hack to invalidate build cache
#ARG kill_build_cache
#
#RUN echo "Invalidating build cache kernel $kill_build_cache"

# Copy kernel bins
COPY --from=sigma-build-kernel /home/sigmaos/bin/kernel /home/sigmaos/bin/kernel
COPY --from=sigma-build-kernel /home/sigmaos/create-net.sh /home/sigmaos/bin/kernel/create-net.sh
# Copy named bin
RUN mkdir -p /home/sigmaos/bin/user/common && \
  cp /home/sigmaos/bin/kernel/named /home/sigmaos/bin/user/common/named
# Copy linux bins
COPY --from=sigma-build-kernel /home/sigmaos/bin/linux /home/sigmaos/bin/linux
CMD ["/bin/sh", "-c", "bin/linux/bootkernel ${kernelid} ${named} ${boot} ${dbip} ${mongoip} ${overlays} ${reserveMcpu}"]

# ========== kernel image, including user binaries ==========
FROM sigmaos AS sigmaos-with-userbin
COPY --from=sigma-build-user /home/sigmaos/bin/user /home/sigmaos/bin/user
RUN cp /home/sigmaos/bin/kernel/named /home/sigmaos/bin/user/common/named

CMD ["/bin/sh", "-c", "bin/linux/bootkernel ${kernelid} ${named} ${boot} ${dbip} ${mongoip} ${overlays} ${reserveMcpu}"]
