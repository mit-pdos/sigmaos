#!/bin/bash

usage() {
  echo "Usage: $0 [--noreboot] --vpc VPC --vm VM_DNS_NAME" 1>&2
}

VPC=""
VM=""
REBOOT="reboot"
while [[ $# -gt 0 ]]
do
  case $1 in
  --noreboot)
    REBOOT="--noreboot"
    shift
    ;;
  --vpc)
    shift
    VPC=$1
    shift
    ;;
  --vm)
    shift
    VM=$1
    shift
    ;;
  -help)
    usage
    exit 0
    ;;
  *)
    echo "Error: unexpected argument '$1'"
    usage
    exit 1
    ;;
  esac
done

echo $0 $REBOOT $VPC $VM

if [ -z "$VPC" ] || [ -z "$VM" ] || [ $# -gt 0 ]; then
  usage
  exit 1
fi

DIR=$(realpath $(dirname $0))
source $DIR/env.sh

KEY="key-$VPC.pem"
LOGIN=ubuntu
if [ $REBOOT = "reboot" ]; then
  # try to deal with lag before instance is created and configured
  echo -n "wait until cloud-init is done "
  
  while true; do
    done=`ssh -n -o ConnectionAttempts=1000 -i key-$VPC.pem $LOGIN@$VM cloud-init status`
    if [ "$done" = "status: done" ]; then
      break
    fi
    echo -n "."
    sleep 1
  done
  
  echo "done; reboot and wait"
  
  ssh -n -i key-$VPC.pem $LOGIN@$VM sudo shutdown -r now
  
  sleep 2
  
  while true; do
    done=`ssh -n -i key-$VPC.pem $LOGIN@$VM echo "this is an ssh"`
    if [ ! -z "$done" ]; then
      break
    fi
    echo -n "."
    sleep 1
  done
  
  echo "done rebooting"
fi

# Set up a few directories, and prepare to scp the aws secrets.
ssh -i key-$VPC.pem $LOGIN@$VM <<ENDSSH
sudo mkdir -p /mnt/9p
mkdir ~/.aws
mkdir ~/.docker
chmod 700 ~/.aws
echo > ~/.aws/credentials
chmod 600 ~/.aws/credentials
ENDSSH

# decrypt the aws and docker secrets.
SECRETS=".aws/credentials .docker/config.json"
for F in $SECRETS
do
  yes | gpg --output $F --decrypt ${F}.gpg || exit 1
done

# scp the aws and docker secrets to the server and remove them locally.
scp -i key-$VPC.pem .aws/config $LOGIN@$VM:/home/$LOGIN/.aws/
scp -i key-$VPC.pem .aws/credentials $LOGIN@$VM:/home/$LOGIN/.aws/
scp -i key-$VPC.pem .docker/config.json $LOGIN@$VM:/home/$LOGIN/.docker/
rm $SECRETS

ssh -i key-$VPC.pem $LOGIN@$VM <<ENDSSH
cat <<EOF > ~/.ssh/config
Host *
  StrictHostKeyChecking no
  UserKnownHostsFile=/dev/null
EOF
chmod go-w .ssh/config

cat << EOF > ~/.ssh/aws-sigmaos
-----BEGIN OPENSSH PRIVATE KEY-----
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
-----END OPENSSH PRIVATE KEY-----
EOF
chmod 600 ~/.ssh/aws-sigmaos
cp ~/.ssh/aws-sigmaos ~/.ssh/id_rsa

cat << EOF >> ~/.ssh/authorized_keys
ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQC4NF0v/XEFId9bJJ1KvzvIIfcFUPvvNJCWH35JJbpaCCRuguHAlim30WqeTG+Ru7Debl80AVuve+XrhL2uYY6R1SeBXQ6Vl6jGPzmmlTqJLi73e6oNWI13QJ1ALriS2Vy5xk1ckmS5epYS0OixerQJ/9gHTcdHWcNDbfUOi23jqdciNExSqjamrYvUwi14IhRNRqltrk2V4ephnRI+8S3ExansbZSwnu0XIz7j86e3PFMuuHwLJWv59UdO9roJl2B36dnzWp0lpqcXYrk3gbbXBCu6iV1Dv7XgvElTtmwqJJ50O2pzwJv2pBB/tw3LkWldF6FuYO3vjaTOgdm2gbCsw2DMJSa6oXJB4cRztXDe51ljbhdYptHxbJgM7+852soEma2uhuek80rRn3UEqrQ1MIsw0DJXx5k+tDbJAWyzy4k4opR583Go9UtRq/BY6qyaFHA/DY13c5QiJNapN5JameX3+wUvNmR22lX/SW61KFjXzYnn//77UCidNPr6SQs= kaashoek@fk6x1
EOF

sudo mkdir -p /mnt/9p

if [ -d "DeathStarBench" ] 
then
  (cd DeathStarBench; git pull;)
else
  git clone https://github.com/mit-pdos/DeathStarBench.git
fi

if [ -d "sigmaos" ] 
then
  ssh-agent bash -c 'ssh-add ~/.ssh/aws-sigmaos; (cd sigmaos; git pull;)'
else
  ssh-agent bash -c 'ssh-add ~/.ssh/aws-sigmaos; git clone git@g.csail.mit.edu:sigmaos; (cd sigmaos; go mod download;)'
  # Indicate that sigma has not been build yet on this instance
  touch ~/.nobuild
  # Load apparmor profile
  (cd sigmaos; sudo apparmor_parser -r scontainer/sigmaos-uproc)
fi

if [ -d "corral" ] 
then
  ssh-agent bash -c 'ssh-add ~/.ssh/aws-sigmaos; (cd corral; git pull;)'
else
  ssh-agent bash -c 'ssh-add ~/.ssh/aws-sigmaos; git clone git@g.csail.mit.edu:corral; (cd corral; git checkout k8s; git pull; go mod download;)'
  # Indicate that sigma has not been build yet on this instance
  touch ~/.nobuild
fi

# Install the latest version of golang
if ! [ -d "go1.22.2.linux-amd64.tar.gz" ] then
    wget https://go.dev/dl/go1.22.2.linux-amd64.tar.gz
    sudo rm -rf /usr/local/go && sudo tar -C /usr/local -xzf go1.22.2.linux-amd64.tar.gz
    echo "export PATH=\$PATH:/usr/local/go/bin" | cat - .bashrc > .bashrc.tmp && mv .bashrc.tmp .bashrc
    echo "export PATH=\$PATH:/usr/local/go/bin" | cat - .profile > .profile.tmp && mv .profile.tmp .profile
    sudo apt remove -y golang-go golang
    go version
fi

# Add to docker group
sudo usermod -aG docker ubuntu

# Increase root's open file ulimits.
echo "root hard nofile 100000" | sudo tee -a /etc/security/limits.conf
echo "root soft nofile 100000" | sudo tee -a /etc/security/limits.conf

# Increase ubuntu user's open file ulimits.
echo "ubuntu hard nofile 100000" | sudo tee -a /etc/security/limits.conf
echo "ubuntu soft nofile 100000" | sudo tee -a /etc/security/limits.conf

echo -n > ~/.hushlogin
ENDSSH

echo "Installing kubernetes components"
ssh -i key-$VPC.pem $LOGIN@$VM <<'ENDSSH'
  sudo apt-get install -y apt-transport-https ca-certificates curl
  sudo mkdir -p /etc/apt/keyrings/
  curl -fsSL https://packages.cloud.google.com/apt/doc/apt-key.gpg | sudo gpg --dearmor -o /etc/apt/keyrings/kubernetes-archive-keyring.gpg
  echo "deb [signed-by=/etc/apt/keyrings/kubernetes-archive-keyring.gpg] https://apt.kubernetes.io/ kubernetes-xenial main" | sudo tee /etc/apt/sources.list.d/kubernetes.list
  sudo apt update
#    sudo apt-mark unhold kubelet kubeadm kubectl
#    sudo apt remove -y kubelet kubeadm kubectl
  sudo apt install -y kubelet kubeadm kubectl
  curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo gpg --dearmor -o /usr/share/keyrings/docker-archive-keyring.gpg
  echo "deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/docker-archive-keyring.gpg] https://download.docker.com/linux/ubuntu $(lsb_release -cs) stable" | sudo tee /etc/apt/sources.list.d/docker.list > /dev/null
  sudo apt update
  yes | sudo apt install docker-ce docker-ce-cli containerd.io
  sudo usermod -aG docker $USER && newgrp docker
  curl https://baltocdn.com/helm/signing.asc | sudo apt-key add -
  sudo apt install apt-transport-https --yes
  echo "deb https://baltocdn.com/helm/stable/debian/ all main" | sudo tee /etc/apt/sources.list.d/helm-stable-debian.list
  sudo apt update
  sudo apt install -y helm
  helm repo add stable https://charts.helm.sh/stable
  sudo swapoff -a
  echo br_netfliter | sudo tee /etc/modules-load.d/k8s.conf
  printf "net.bridge.bridge-nf-call-ip6tables = 1\nnet.bridge.bridge-nf-call-iptables = 1" | sudo tee /etc/sysctl.d/k8s.conf
  sudo sysctl --system
  printf "{\n\"exec-opts\": [\"native.cgroupdriver=systemd\"]\n}" | sudo tee /etc/docker/daemon.json
  sudo systemctl daemon-reload
  sudo systemctl restart docker
  sudo systemctl restart kubelet
  sudo containerd config default | sudo tee /etc/containerd/config.toml
  sudo sed -i 's/            SystemdCgroup = false/            SystemdCgroup = true/' /etc/containerd/config.toml
  sudo systemctl daemon-reload
  sudo systemctl restart docker
  sudo systemctl restart containerd
  sudo systemctl restart kubelet
  sudo systemctl restart containerd
  sudo groupadd docker
  sudo usermod -aG docker ubuntu
  sudo usermod -aG docker ubuntu
  # For DeathStarBench
  sudo apt install -y docker-compose luarocks libssl-dev zlib1g-dev
  sudo luarocks install luasocket

  sudo swapoff -a
  sudo sed -i '/\tswap\t/ s/^/#/' /etc/fstab
ENDSSH

echo "Done setting up instance $VM"
