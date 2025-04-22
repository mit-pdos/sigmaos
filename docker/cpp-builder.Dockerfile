# syntax=docker/dockerfile:1-experimental

FROM archlinux

RUN pacman-key --init
RUN pacman-key --refresh-keys
RUN pacman-key -u
RUN pacman-key --populate

RUN pacman --noconfirm -Sy archlinux-keyring

RUN pacman --noconfirm -Sy git libseccomp wget gcc pkg-config parallel time make cmake

WORKDIR /home/sigmaos

CMD [ "/bin/bash", "-l" ]
