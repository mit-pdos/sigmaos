# syntax=docker/dockerfile:1-experimental

FROM archlinux
ARG APP_NAME

RUN yes | pacman -Sy git libseccomp wget gcc pkg-config

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

WORKDIR /app
COPY ./make-app.sh ./make.sh

COPY ${APP_NAME}/go.* .
RUN /go-custom/bin/go mod download

COPY ${APP_NAME} .

# Build all binaries.
RUN --mount=type=cache,target=/root/.cache/go-build ./make.sh --parallel --gopath /go-custom/bin/go
