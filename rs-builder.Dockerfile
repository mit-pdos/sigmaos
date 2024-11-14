# syntax=docker/dockerfile:1-experimental

FROM alpine

RUN apk add --no-cache libseccomp \
  gcompat \
  musl-dev \
  curl \
  bash \
  gcc \
  libc-dev \
  parallel \
  libseccomp-static \
  make

RUN echo 'will cite' | parallel --citation || true

WORKDIR /home/sigmaos
RUN mkdir -p bin/kernel && \
  mkdir -p bin/user

# Install Python
# RUN wget https://www.python.org/ftp/python/3.11.0/Python-3.11.0.tar.xz && tar -xJf Python-3.11.0.tar.xz
# COPY pyModule.config pyModule.config
# RUN cd cpython3.11 && \
#   cat ../pyModule.config >> Modules/Setup && \
#   ./configure --disable-shared && \
#   make -j

# Install rust
RUN curl https://sh.rustup.rs -sSf | bash -s -- -y
RUN echo 'source $HOME/.cargo/env' >> $HOME/.bashrc
RUN source $HOME/.bashrc

# Copy rust trampoline
COPY rs rs 
ENV LIBSECCOMP_LINK_TYPE=static
ENV LIBSECCOMP_LIB_PATH="/usr/lib"

CMD [ "/bin/bash", "-l" ]
