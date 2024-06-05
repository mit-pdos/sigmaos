#!/bin/bash

if [ "$#" -ne 1 ]
then
  echo "Usage: $0 address"
  exit 1
fi

DIR=$(dirname $0)
source $DIR/env.sh

echo "Installing kubernetes components"
ssh -i $DIR/keys/cloudlab-sigmaos $LOGIN@$1 <<'ENDSSH'
  sudo apt-get install -y apt-transport-https ca-certificates curl
  sudo mkdir -p /etc/apt/keyrings/
  curl -fsSL https://pkgs.k8s.io/core:/stable:/v1.29/deb/Release.key | sudo gpg --dearmor -o /etc/apt/keyrings/kubernetes-apt-keyring.gpg
  sudo chmod 644 /etc/apt/keyrings/kubernetes-apt-keyring.gpg # allow unprivileged APT programs to read this keyring
  echo 'deb [signed-by=/etc/apt/keyrings/kubernetes-apt-keyring.gpg] https://pkgs.k8s.io/core:/stable:/v1.29/deb/ /' | sudo tee /etc/apt/sources.list.d/kubernetes.list
  sudo chmod 644 /etc/apt/sources.list.d/kubernetes.list   # helps tools such as command-not-found to work correctly
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
  sudo usermod -aG docker arielck
  sudo usermod -aG docker arielck
  # For DeathStarBench
  sudo apt install -y docker-compose luarocks libssl-dev zlib1g-dev
  sudo luarocks install luasocket

  # Move containerd to the local drive
  sudo systemctl stop containerd
  sudo rm -rf /var/local/arielck/containerd
  sudo mv /var/lib/containerd /var/local/arielck
  #sudo rm /etc/containerd/config.toml
  #echo 'root = "/var/local/arielck/containerd"' | sudo tee /etc/containerd/config.toml
  sudo sed -i 's/\/var\/lib\/containerd/\/var\/local\/arielck\/containerd/g' /etc/containerd/config.toml
  sudo systemctl restart containerd
  
  sudo rm -rf /var/local/arielck/docker
  sudo mv /var/lib/docker /var/local/arielck
  sudo sed -i 's/ExecStart=\/usr\/bin\/dockerd/ExecStart=\/usr\/bin\/dockerd --data-root \/var\/local\/arielck\/docker/g' /lib/systemd/system/docker.service
  sudo systemctl daemon-reload
  sudo systemctl restart docker
  
  sudo swapoff -a
  sudo sed -i '/\tswap\t/ s/^/#/' /etc/fstab
ENDSSH
