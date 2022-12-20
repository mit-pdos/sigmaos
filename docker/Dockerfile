FROM golang:1.19.4-bullseye 
WORKDIR /ulambda

# Copy go mod files.
COPY go.* .
RUN go mod download

# Install some packages
RUN apt-get update && apt-get install -y openssh-server

# Copy full src
COPY . .

# Set up user.
RUN useradd -rm -d /home/ubuntu -s /bin/bash -g root -G sudo -u 1000 ubuntu
RUN echo 'ubuntu:pwd' | chpasswd
RUN mkdir -p /home/ubuntu/.ssh
RUN cat id_rsa.pub > /home/ubuntu/.ssh/authorized_keys

# Set up SSH.
RUN mkdir /var/run/sshd
RUN service ssh start
EXPOSE 22

# Build sigmaos
RUN ./make.sh --norace --version TEST && ./install.sh --realm test-ream --version TEST && ./install.sh --realm test-realm --version TEST
 
CMD ["/usr/sbin/sshd", "-D"]
