#!/bin/bash

#
# Download and install a root file system for sigmaos in default
# location, which requires building sigmaos so that we can use
# `contain` to initialize image.
#

DIR=$(dirname $0)
. $DIR/env/env.sh

if [ -z "$SIGMAROOTFS" ] ; then
    echo "no environment variables SIGMAROOTFS"
    exit 1
fi

TAR=rootfs.tar.zst
REALM=test-realm

mkdir -p rootfs

# is this latest?
wget -O rootfs/$TAR  https://gitlab.archlinux.org/archlinux/archlinux-docker/-/package_files/3798/download || exit 1

(cd rootfs && tar --use-compress-program=unzstd -xf $TAR && rm $TAR) || exit 1

# for DNS
cp /etc/resolv.conf rootfs/etc/resolv.conf || exit 1

# for containers
mkdir -p rootfs/proc
mkdir -p rootfs/dev
mkdir -p rootfs/sys
echo -n > rootfs/dev/urandom
echo -n > rootfs/dev/null

# put rootfs in place
mv rootfs $SIGMAROOTFS || exit 1

exit 1

# build sigmaos
./make.sh --no-race   || exit 1

# install sigmaos in rootfs
./install.sh --realm $REALM  || exit 1

# install setuid scnet
echo "install-scnet requires sudo to install into /usr/sbin/scnet"
./install-scnet.sh || exit 1

# set path to find contain
source env/init.sh

# install packages in rootfs
contain pacman-db-upgrade
contain pacman -Sy
contain pacman -S go

# sanity check
go test -v sigmaos/fslib --version=$(cat VERSION.txt) -run InitFs

