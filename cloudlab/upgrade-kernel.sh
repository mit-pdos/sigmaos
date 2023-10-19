#!/bin/bash

if [ "$#" -ne 1 ]
then
  echo "Usage: $0 address"
  exit 1
fi

DIR=$(dirname $0)
source $DIR/env.sh

SSHCMD="$LOGIN@$1"

{ ssh -i $DIR/keys/cloudlab-sigmaos $SSHCMD <<ENDSSH
sudo apt update
sudo apt install -y flex bison

cd /var/local/$LOGIN
mkdir kernel

cd kernel
mkdir kbuild-$KERNEL
wget https://cdn.kernel.org/pub/linux/kernel/v$(printf %.1s "$KERNEL").x/linux-$KERNEL.tar.xz
tar -xvf linux-$KERNEL.tar.xz

cd /var/local/$LOGIN/kernel/kbuild-$KERNEL
yes "" | make -C ../linux-$KERNEL O=/var/local/$LOGIN/kernel/kbuild-$KERNEL config
sed -ri '/CONFIG_SYSTEM_TRUSTED_KEYS/s/=.+/=""/g' .config
sed -ri 's/CONFIG_SATA_AHCI=m/CONFIG_SATA_AHCI=y/g' .config
sed -ri 's/CONFIG_SYSTEM_REVOCATION_LIST=y/CONFIG_SYSTEM_REVOCATION_LIST=n/g' .config
sudo apt install -y dwarves
sudo make -j$(nproc) 
INSTALL_MOD_STRIP=1 sudo make modules_install -j$(nproc)
INSTALL_MOD_STRIP=1 sudo make install -j$(nproc)
sudo make -j$(nproc) 
INSTALL_MOD_STRIP=1 sudo make modules_install -j$(nproc)
INSTALL_MOD_STRIP=1 sudo make install -j$(nproc)
#sudo reboot
ENDSSH
} || true # Ignore error from broken SSH connection due to reboot.

echo "Success!"
