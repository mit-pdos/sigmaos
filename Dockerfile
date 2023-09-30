# syntax=docker/dockerfile:1-experimental

FROM alpine as base
ARG tag

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

RUN apk add --no-cache libseccomp gcompat libpthread-stubs musl-dev

WORKDIR /home/sigmaos
RUN mkdir bin && \
    mkdir bin/user && \
    mkdir bin/kernel && \
    mkdir bin/linux
# Copy some yaml files to the base image.
COPY seccomp seccomp
ENV SIGMATAG=$tag

# ========== user image ==========
FROM base AS sigmauser

RUN apk add --no-cache curl bash gcc libc-dev libseccomp-static

# Install rust
RUN curl https://sh.rustup.rs -sSf | bash -s -- -y
RUN echo 'source $HOME/.cargo/env' >> $HOME/.bashrc
RUN source $HOME/.bashrc

RUN mkdir jail

# Copy rust trampoline
COPY exec-uproc-rs exec-uproc-rs
ENV LIBSECCOMP_LINK_TYPE=static
ENV LIBSECCOMP_LIB_PATH="/usr/lib"
RUN (cd exec-uproc-rs && $HOME/.cargo/bin/cargo build)
RUN cp exec-uproc-rs/target/debug/exec-uproc-rs bin/kernel

# Copy mr yaml files.
COPY mr mr
# Copy uprocd, the entrypoint for this container, to the user image.
COPY --from=sigmabuilder /home/sigmaos/bin/kernel/uprocd /home/sigmaos/bin/kernel
# Copy exec-uproc, the trampoline program, to the user image, 
COPY --from=sigmabuilder /home/sigmaos/bin/user/common/exec-uproc /home/sigmaos/bin/kernel

# ========== kernel image, omitting user binaries ==========
FROM base AS sigmakernelclean
WORKDIR /home/sigmaos
ENV kernelid kernel
ENV named ""
ENV boot named
ENV dbip x.x.x.x
ENV mongoip x.x.x.x
ENV overlays "false"
# Install docker-cli
RUN apk add --update docker openrc
ENV reserveMcpu "0"
# Copy kernel bins
COPY --from=sigmabuilder /home/sigmaos/bin/kernel /home/sigmaos/bin/kernel
COPY --from=sigmabuilder /home/sigmaos/create-net.sh /home/sigmaos/bin/kernel/create-net.sh
# Copy linus bins
COPY --from=sigmabuilder /home/sigmaos/bin/linux /home/sigmaos/bin/linux
CMD ["/bin/sh", "-c", "bin/linux/bootkernel ${kernelid} ${named} ${boot} ${dbip} ${mongoip} ${overlays} ${reserveMcpu}"]

# ========== kernel image, including user binaries ==========
FROM sigmakernelclean AS sigmakernel
COPY --from=sigmabuilder /home/sigmaos/bin/user /home/sigmaos/bin/user
