# syntax=docker/dockerfile:1-experimental

FROM ubuntu:24.04

#RUN pacman --noconfirm -Syu
#RUN pacman --noconfirm -Sy git libseccomp wget gcc pkg-config parallel time
RUN apt update && \
  apt install -y git libseccomp-dev wget gcc pkg-config parallel time curl

# Download an initial version of Go
RUN wget "https://go.dev/dl/go1.22.2.linux-amd64.tar.gz" && \
  tar -C /usr/local -xzf go1.22.2.linux-amd64.tar.gz

# Set the PATH to include the new Go install.
ENV PATH="${PATH}:/usr/local/go/bin"

# Install custom version of go with larger minimum stack size.
RUN git clone https://github.com/ArielSzekely/go.git go-custom && \
  cd go-custom && \
  git checkout bigstack-go1.22 && \
  git config pull.rebase false && \
  git pull && \
  cd src && \
  ./make.bash && \
  /go-custom/bin/go version

RUN curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh

WORKDIR /home/sigmaos

CMD [ "/bin/bash", "-l" ]
