# syntax=docker/dockerfile:1-experimental

FROM alpine

RUN apk add --no-cache libseccomp \
  gcompat \
  libpthread-stubs \
  musl-dev \
  curl \
  bash \
  gcc \
  libc-dev \
  libseccomp-static \
  make

# Install musl libc
# RUN wget http://www.musl-libc.org/releases/musl-1.2.4.tar.gz && tar -xzf musl-1.2.4.tar.gz
# RUN cd musl-1.2.4 && \
#   ./configure --disable-shared && \
#   make -j && \
#   make install && ls /usr/local/musl

#ENV CC=musl-gcc

# Install python
# RUN git clone https://github.com/python/cpython.git
#   git checkout tags/v3.12.0 && \

WORKDIR /home/sigmaos
RUN mkdir -p bin/kernel && \
  mkdir -p bin/user

RUN wget https://www.python.org/ftp/python/3.5.0/Python-3.5.0.tar.xz && tar -xJf Python-3.5.0.tar.xz
RUN cd Python-3.5.0 && \
  ./configure --disable-shared LDFLAGS="-static" CFLAGS="-static" CPPFLAGS="-static" && \
  make -j

RUN cp Python-3.5.0/python bin/user

# Install rust
RUN curl https://sh.rustup.rs -sSf | bash -s -- -y
RUN echo 'source $HOME/.cargo/env' >> $HOME/.bashrc
RUN source $HOME/.bashrc

# Copy rust trampoline
COPY exec-uproc-rs exec-uproc-rs
ENV LIBSECCOMP_LINK_TYPE=static
ENV LIBSECCOMP_LIB_PATH="/usr/lib"
RUN (cd exec-uproc-rs && $HOME/.cargo/bin/cargo build)
RUN cp exec-uproc-rs/target/debug/exec-uproc-rs bin/kernel

RUN touch /home/sigmaos/bin/user/test-rust-bin

# When this container image is run, copy bins to host
CMD ["sh", "-c", "cp -r bin/user/* /tmp/bin/common/"]
