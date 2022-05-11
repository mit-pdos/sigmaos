#!/bin/bash

if [ "$#" -ne 1 ]
then
  echo "Usage: $0 user@address"
  exit 1
fi

echo "$0 $1"

DIR=$(dirname $0)
BLKDEV=/dev/sda4

. $DIR/config

# Set up bash as the primary shell
ssh -i $DIR/keys/cloudlab-sigmaos $1 <<ENDSSH
sudo chsh -s /bin/bash arielck
ENDSSH

ssh -i $DIR/keys/cloudlab-sigmaos $1 <<ENDSSH
sudo mkfs -t ext4 $BLKDEV
sudo mount $BLKDEV /var/local
sudo mkdir /var/local/$USER
sudo chown $USER /var/local/$USER

sudo blkid $BLKDEV | cut -d \" -f2

cd /var/local/$USER
mkdir kernel

cd kernel
mkdir kbuild
wget https://cdn.kernel.org/pub/linux/kernel/v5.x/linux-5.14.14.tar.xz
tar -xvf linux-5.14.14.tar.xz

cd kbuild
yes "" | make -C ../linux-5.14.14 O=/var/local/$USER/kernel/kbuild config
sed -ri '/CONFIG_SYSTEM_TRUSTED_KEYS/s/=.+/=""/g' .config
sed -ri 's/CONFIG_SATA_AHCI=m/CONFIG_SATA_AHCI=y/g' .config
sed -ri 's/CONFIG_SYSTEM_REVOCATION_LIST=y/CONFIG_SYSTEM_REVOCATION_LIST=n/g' .config
sudo make -j8 
INSTALL_MOD_STRIP=1 sudo make -j8 modules_install
INSTALL_MOD_STRIP=1 sudo make -j8 install
sudo reboot

ENDSSH

echo "== TO LOGIN TO VM INSTANCE USE: =="
echo "ssh $1"
echo "============================="

