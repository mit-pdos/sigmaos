# syntax=docker/dockerfile:1-experimental

FROM archlinux

RUN yes | pacman -Syu
RUN yes | pacman -Sy git libseccomp wget gcc pkg-config parallel time

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

## Set env after downlaoding and installing the custom Go version, so we don't
## rebuild Go evey time we change tags.
#ENV SIGMATAG=$tag
#
## Copy source
#COPY . .
## Build kernel binaries.
#RUN --mount=type=cache,target=/root/.cache/go-build ./make.sh --norace --gopath /go-custom/bin/go --target $target $parallel kernel && \
#  mkdir bin/user/common 
CMD [ "/bin/bash", "-l" ]
