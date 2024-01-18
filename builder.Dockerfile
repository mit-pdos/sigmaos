# syntax=docker/dockerfile:1-experimental

FROM archlinux

RUN pacman -Syu --noconfirm
RUN pacman -Sy --noconfirm git libseccomp wget gcc pkg-config parallel time

# Download an initial version of Go
RUN wget "https://go.dev/dl/go1.20.4.linux-amd64.tar.gz" && \
  tar -C /usr/local -xzf go1.20.4.linux-amd64.tar.gz

# Set the PATH to include the new Go install.
ENV PATH="${PATH}:/usr/local/go/bin"

# Install custom version of go with larger minimum stack size.
RUN git clone https://github.com/ArielSzekely/go.git go-custom && \
  cd go-custom && \
  git checkout bigstack && \
  git config pull.rebase false && \
  git pull && \
  cd src && \
  ./make.bash

WORKDIR /home/sigmaos

CMD [ "/bin/bash", "-l" ]
