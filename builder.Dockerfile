# syntax=docker/dockerfile:1-experimental

FROM archlinux

#RUN pacman --noconfirm -Syu
RUN pacman-key --init && \
  pacman-key --refresh-keys && \
  pacman-key -u && \
  pacman-key --populate && \
  pacman --noconfirm -Sy archlinux-keyring

RUN pacman --noconfirm -Sy git libseccomp wget gcc pkg-config parallel time make cmake protobuf=30.2 spdlog

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

# # Install specific version of OpenBLAS
# RUN wget -P / https://github.com/xianyi/OpenBLAS/releases/download/v0.3.23/OpenBLAS-0.3.23.tar.gz && \
#   tar -xzf /OpenBLAS-0.3.23.tar.gz && \
#   rm /OpenBLAS-0.3.23.tar.gz && \
#   cd /OpenBLAS-0.3.23 && \
#   make USE_THREAD=1 INTERFACE64=1 DYNAMIC_ARCH=1 SYMBOLSUFFIX=64_

# Install Python
RUN git clone https://github.com/ivywu2003/cpython.git /cpython3.11 && \
  cd /cpython3.11 && \ 
  git checkout 3.11 && \
  git config pull.rebase false && \
  git pull && \
  ./configure --prefix=/home/sigmaos/bin/user --exec-prefix=/home/sigmaos/bin/user && \
  make -j

WORKDIR /home/sigmaos

# Copy python user programs
COPY pyproc pyproc

CMD [ "/bin/bash", "-l" ]
