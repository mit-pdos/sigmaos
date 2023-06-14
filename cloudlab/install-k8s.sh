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
  bash -c "sudo apt-get install -y apt-transport-https ca-certificates curl"
  bash -c "curl -fsSL https://packages.cloud.google.com/apt/doc/apt-key.gpg | sudo gpg --dearmor -o /etc/apt/keyrings/kubernetes-archive-keyring.gpg"
  bash -c "echo \"deb [signed-by=/etc/apt/keyrings/kubernetes-archive-keyring.gpg] https://apt.kubernetes.io/ kubernetes-xenial main\" | sudo tee /etc/apt/sources.list.d/kubernetes.list"
  bash -c "sudo apt update"
#    bash -c "sudo apt-mark unhold kubelet kubeadm kubectl"
#    bash -c "sudo apt remove -y kubelet kubeadm kubectl"
  bash -c "sudo apt install -y kubelet kubeadm kubectl"
  bash -c "curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo gpg --dearmor -o /usr/share/keyrings/docker-archive-keyring.gpg"
  bash -c "echo \"deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/docker-archive-keyring.gpg] https://download.docker.com/linux/ubuntu $(lsb_release -cs) stable\" | sudo tee /etc/apt/sources.list.d/docker.list > /dev/null"
  bash -c "sudo apt update"
  bash -c "yes | sudo apt install docker-ce docker-ce-cli containerd.io"
  bash -c "sudo usermod -aG docker $USER && newgrp docker"
  bash -c "curl https://baltocdn.com/helm/signing.asc | sudo apt-key add -"
  bash -c "sudo apt install apt-transport-https --yes"
  bash -c "echo \"deb https://baltocdn.com/helm/stable/debian/ all main\" | sudo tee /etc/apt/sources.list.d/helm-stable-debian.list"
  bash -c "sudo apt update"
  bash -c "sudo apt install -y helm"
  bash -c "helm repo add stable https://charts.helm.sh/stable"
  bash -c "echo br_netfliter | sudo tee /etc/modules-load.d/k8s.conf"
  bash -c "printf \"net.bridge.bridge-nf-call-ip6tables = 1\nnet.bridge.bridge-nf-call-iptables = 1\" | sudo tee /etc/sysctl.d/k8s.conf"
  bash -c "sudo sysctl --system"
  bash -c 'printf "{\n\"exec-opts\": [\"native.cgroupdriver=systemd\"]\n}" | sudo tee /etc/docker/daemon.json'
  bash -c "sudo systemctl daemon-reload"
  bash -c "sudo systemctl restart docker"
  bash -c "sudo systemctl restart kubelet"
  bash -c "sudo containerd config default | sudo tee /etc/containerd/config.toml"
  bash -c "sudo sed -i 's/            SystemdCgroup = false/            SystemdCgroup = true/' /etc/containerd/config.toml"
  bash -c "sudo systemctl daemon-reload"
  bash -c "sudo systemctl restart docker"
  bash -c "sudo systemctl restart containerd"
  bash -c "sudo systemctl restart kubelet"
  bash -c "sudo systemctl restart containerd"
  bash -c "sudo groupadd docker"
  bash -c "sudo usermod -aG docker arielck"
  bash -c "sudo usermod -aG docker arielck"
  # For DeathStarBench
  bash -c "sudo apt install -y docker-compose luarocks libssl-dev zlib1g-dev"
  bash -c "sudo luarocks install luasocket"

  # Move containerd to the local drive
  sudo systemctl stop containerd
  sudo rm -rf /var/local/arielck/containerd
  sudo mv /var/lib/containerd /var/local/arielck
  #sudo rm /etc/containerd/config.toml
  #echo 'root = "/var/local/arielck/containerd"' | sudo tee /etc/containerd/config.toml
  bash -c "sudo sed -i 's/\/var\/lib\/containerd/\/var\/local\/arielck\/containerd/g' /etc/containerd/config.toml"
  sudo systemctl restart containerd
  
  sudo rm -rf /var/local/arielck/docker
  sudo mv /var/lib/docker /var/local/arielck
  sudo sed -i 's/ExecStart=\/usr\/bin\/dockerd/ExecStart=\/usr\/bin\/dockerd --data-root \/var\/local\/arielck\/docker/g' /lib/systemd/system/docker.service
  sudo systemctl daemon-reload
  sudo systemctl restart docker
  
  sudo swapoff -a
  sudo sed -i '/\tswap\t/ s/^/#/' /etc/fstab
ENDSSH
