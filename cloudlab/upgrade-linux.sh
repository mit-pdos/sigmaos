#!/bin/bash

if [ "$#" -ne 3 ]
then
  echo "Usage: $0 user address blkdev"
  exit 1
fi

LOGIN=$1
ADDR=$2
SSHCMD="$1@$2"

DIR=$(dirname $0)
BLKDEV=$3
KERNEL=6.1.24

. $DIR/config

# Set up bash as the primary shell
ssh -i $DIR/keys/cloudlab-sigmaos $SSHCMD <<ENDSSH
sudo chsh -s /bin/bash $LOGIN
ENDSSH

ssh -i $DIR/keys/cloudlab-sigmaos $SSHCMD <<ENDSSH
echo "##### $0: user is $LOGIN; block is $BLKDEV; kernel version is $KERNEL. #####"
sudo mkfs -t ext4 $BLKDEV
sudo mount $BLKDEV /var/local
sudo mkdir /var/local/$LOGIN
sudo chown $LOGIN /var/local/$LOGIN

sudo blkid $BLKDEV | cut -d \" -f2
echo -e UUID=$(sudo blkid $BLKDEV | cut -d \" -f2)'\t/var/local\text4\tdefaults\t0\t2' | sudo tee -a /etc/fstab

# Set max journal size
sudo journalctl --vacuum-size=100M

sudo apt update
sudo apt install libelf-dev

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
sudo make -j\$(nproc) 
INSTALL_MOD_STRIP=1 sudo make modules_install -j\$(nproc)
INSTALL_MOD_STRIP=1 sudo make install -j\$(nproc)
sudo make -j\$(nproc) 
INSTALL_MOD_STRIP=1 sudo make modules_install -j\$(nproc)
INSTALL_MOD_STRIP=1 sudo make install -j\$(nproc)
sudo reboot

ENDSSH

echo "== TO LOGIN TO VM INSTANCE USE: =="
echo "ssh $SSHCMD"
echo "============================="

