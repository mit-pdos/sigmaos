# syntax=docker/dockerfile:1-experimental

FROM archlinux

RUN pacman-key --init && \
  pacman-key --refresh-keys && \
  pacman-key -u && \
  pacman-key --populate && \
  pacman --noconfirm -Sy archlinux-keyring

RUN pacman --noconfirm -Sy git libseccomp wget gcc pkg-config parallel time make cmake protobuf spdlog

WORKDIR /home/sigmaos

CMD [ "/bin/bash", "-l" ]
