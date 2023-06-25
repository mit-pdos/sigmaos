#!/bin/bash

if [ "$#" -ne 1 ]
then
  echo "Usage: $0 address"
  exit 1
fi

DIR=$(dirname $0)
source $DIR/env.sh

SSHCMD="$LOGIN@$1"

KERNEL=6.1.24

# Set up bash as the primary shell
ssh -i $DIR/keys/cloudlab-sigmaos $SSHCMD <<ENDSSH
sudo chsh -s /bin/bash $LOGIN
ENDSSH

ssh -i $DIR/keys/cloudlab-sigmaos $SSHCMD <<ENDSSH
echo "##### $0: user is $LOGIN; block is $BLKDEV; kernel version is $KERNEL. #####"
sudo mkfs -t ext4 $BLKDEV
sudo mkdir /data
sudo mount $BLKDEV /data
sudo mount $BLKDEV /var/local
sudo mkdir /var/local/$LOGIN
sudo chown $LOGIN /var/local/$LOGIN
sudo blkid $BLKDEV | cut -d \" -f2
ENDSSH

# Use envsubst to ensure the "sudo blkid ...." isn't run locally, but rather is
# run on the remote machine.
CMD=$(
envsubst '$BLKDEV' <<'ENDSSH'
  echo -e UUID=$(sudo blkid $BLKDEV | cut -d \" -f2)'\t/var/local\text4\tdefaults\t0\t2' | sudo tee -a /etc/fstab
ENDSSH
)
ssh -i $DIR/keys/cloudlab-sigmaos $SSHCMD <<ENDSSH
  $CMD
ENDSSH

ssh -i $DIR/keys/cloudlab-sigmaos $SSHCMD <<ENDSSH
# Set max journal size
sudo journalctl --vacuum-size=100M

sudo apt update
sudo apt install libelf-dev

cd /var/local/$LOGIN
#mkdir kernel
#
#cd kernel
#mkdir kbuild-$KERNEL
#wget https://cdn.kernel.org/pub/linux/kernel/v$(printf %.1s "$KERNEL").x/linux-$KERNEL.tar.xz
#tar -xvf linux-$KERNEL.tar.xz
#
#cd /var/local/$USER/kernel/kbuild-$KERNEL
#yes "" | make -C ../linux-$KERNEL O=/var/local/$USER/kernel/kbuild-$KERNEL config
#sed -ri '/CONFIG_SYSTEM_TRUSTED_KEYS/s/=.+/=""/g' .config
#sed -ri 's/CONFIG_SATA_AHCI=m/CONFIG_SATA_AHCI=y/g' .config
#sed -ri 's/CONFIG_SYSTEM_REVOCATION_LIST=y/CONFIG_SYSTEM_REVOCATION_LIST=n/g' .config
#sudo apt install -y dwarves
#sudo make -j$(nproc) 
#INSTALL_MOD_STRIP=1 sudo make modules_install -j$(nproc)
#INSTALL_MOD_STRIP=1 sudo make install -j$(nproc)
#sudo make -j$(nproc) 
#INSTALL_MOD_STRIP=1 sudo make modules_install -j$(nproc)
#INSTALL_MOD_STRIP=1 sudo make install -j$(nproc)
#sudo reboot
ENDSSH

# Run in heredoc without variable expansion to ensure $(uname -r) isn't run on
# the local machine.
ssh -i $DIR/keys/cloudlab-sigmaos $SSHCMD <<"ENDSSH"
sudo apt update
sudo apt install -y linux-tools-$(uname -r)
sudo apt install -y linux-tools-common
ENDSSH

ssh -i $DIR/keys/cloudlab-sigmaos $SSHCMD <<ENDSSH
# Disable automatic frequency-scaling and switch off cstates
sudo sed -i s/GRUB_CMDLINE_LINUX_DEFAULT=.*/GRUB_CMDLINE_LINUX_DEFAULT="\"intel_pstate=passive intel_idle.max_cstate=0 systemd.unified_cgroup_hierarchy=1\""/g  /etc/default/grub
sudo update-grub
sudo reboot
ENDSSH
