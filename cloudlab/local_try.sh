sudo apt update
  sudo apt install -y gcc \
  make \
  gcc-7 \
  gcc \
  g++-7 \
  g++ \
  protobuf-compiler \
  libprotobuf-dev \
  libcrypto++-dev \
  python3 \
  libcap-dev \
  libncurses5-dev \
  libboost-dev \
  libssl-dev \
  autopoint \
  help2man \
  texinfo \
  automake \
  libtool \
  pkg-config \
  libhiredis-dev \
  python3-boto3 \
  ffmpeg \
  htop \
  net-tools \
  libprotoc-dev \
  libssl-dev \
  git-lfs \
  libseccomp-dev \
  awscli \
  htop \
  jq \
  docker.io \
  mysql-client

  # For hadoop
#  yes | sudo apt install openjdk-8-jdk \
#  openjdk-8-jre-headless

  wget 'https://golang.org/dl/go1.20.3.linux-amd64.tar.gz'
  sudo rm -rf /usr/local/go && sudo tar -C /usr/local -xzf go1.20.3.linux-amd64.tar.gz
  export PATH=/bin:/sbin:/usr/sbin:\$PATH:/usr/local/go/bin
  echo "PATH=\$PATH:/usr/local/go/bin" >> ~/.profile
  go version

