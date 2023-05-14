#!/bin/bash

if [ "$#" -ne 1 ]
then
  echo "Usage: $0 user@address"
  exit 1
fi

echo "$0 $1"

DIR=$(dirname $0)
BLKDEV=/dev/sda4
KERNEL=6.1.24

. $DIR/config

# Set up bash as the primary shell
ssh -i $DIR/keys/cloudlab-sigmaos $1 <<ENDSSH
sudo chsh -s /bin/bash arielck
ENDSSH

ssh -i $DIR/keys/cloudlab-sigmaos $1 <<'ENDSSH'
BLKDEV=/dev/sda4
KERNEL=6.1.24
export USER=arielck
sudo mkfs -t ext4 $BLKDEV
sudo mount $BLKDEV /var/local
sudo mkdir /var/local/$USER
sudo chown $USER /var/local/$USER

sudo blkid $BLKDEV | cut -d \" -f2
echo -e UUID=$(sudo blkid $BLKDEV | cut -d \" -f2)'\t/var/local\text4\tdefaults\t0\t2' | sudo tee -a /etc/fstab

# Set max journal size
sudo journalctl --vacuum-size=100M

sudo apt update
sudo apt install libelf-dev

cd /var/local/$USER
mkdir kernel

cd kernel
mkdir kbuild-$KERNEL
wget https://cdn.kernel.org/pub/linux/kernel/v$(printf %.1s "$KERNEL").x/linux-$KERNEL.tar.xz
tar -xvf linux-$KERNEL.tar.xz

cd /var/local/$USER/kernel/kbuild-$KERNEL
yes "" | make -C ../linux-$KERNEL O=/var/local/$USER/kernel/kbuild-$KERNEL config
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
sudo reboot

ENDSSH

echo "== TO LOGIN TO VM INSTANCE USE: =="
echo "ssh $1"
echo "============================="

