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
REALM=rootrealm

mkdir -p rootfs

# is this latest?
wget -O rootfs/$TAR  https://gitlab.archlinux.org/archlinux/archlinux-docker/-/package_files/3798/download || exit 1

(cd rootfs && tar --use-compress-program=unzstd -xf $TAR && rm $TAR) || exit 1

# for DNS
echo "nameserver 8.8.8.8" > rootfs/etc/resolv.conf || exit 1

# for containers
mkdir -p rootfs/proc
mkdir -p rootfs/dev
mkdir -p rootfs/sys
echo -n > rootfs/dev/urandom
echo -n > rootfs/dev/null

# put rootfs in place
mkdir -p $SIGMAROOTFS || exit 1
rm -rf $SIGMAROOTFS || exit 1
mv rootfs $SIGMAROOTFS || exit 1

# build sigmaos
./make.sh --norace || exit 1

./install-aws-cred.sh

# install sigmaos in rootfs
./install.sh --realm $REALM  || exit 1

# install setuid scnet
echo "install-scnet requires sudo to install into /usr/sbin/scnet"
./install-scnet.sh || exit 1

# update hosts's routing tables
./iptables.sh

# set path to find contain
source env/init.sh

# install packages in rootfs
contain $REALM pacman-db-upgrade || exit 1
contain $REALM pacman -Sy  || exit 1
contain $REALM pacman -S go

# sanity check
go test -v sigmaos/fslib --run InitFs

echo "run `source env/init.sh` to set SIGMAPATH"
