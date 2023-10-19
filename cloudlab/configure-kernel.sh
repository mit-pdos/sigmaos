#!/bin/bash

if [ "$#" -ne 1 ]
then
  echo "Usage: $0 address"
  exit 1
fi

DIR=$(dirname $0)
source $DIR/env.sh

SSHCMD="$LOGIN@$1"

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
ENDSSH

# Run in heredoc without variable expansion to ensure $(uname -r) isn't run on
# the local machine.
ssh -i $DIR/keys/cloudlab-sigmaos $SSHCMD <<"ENDSSH"
sudo apt update
sudo apt install -y linux-tools-$(uname -r)
sudo apt install -y linux-tools-common
ENDSSH

{ ssh -i $DIR/keys/cloudlab-sigmaos $SSHCMD <<ENDSSH
# Disable automatic frequency-scaling and switch off cstates
sudo sed -i s/GRUB_CMDLINE_LINUX_DEFAULT=.*/GRUB_CMDLINE_LINUX_DEFAULT="\"intel_pstate=passive intel_idle.max_cstate=0 systemd.unified_cgroup_hierarchy=1\""/g  /etc/default/grub
sudo update-grub
sudo reboot
ENDSSH
} || true # Ignore error from broken SSH connection due to reboot.

echo "Success!"
