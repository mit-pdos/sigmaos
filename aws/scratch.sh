#!/bin/bash

usage() {
  echo "Usage: $0 --vpc VPC [--n N] [--taint N:M] [--parallel]" 1>&2
}

VPC=""
PARALLEL=""
while [[ $# -gt 0 ]]; do
  key="$1"
  case $key in
  --vpc)
    shift
    VPC=$1
    shift
    ;;
  --parallel)
    shift
    PARALLEL="true"
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

if [ -z "$VPC" ] || [ $# -gt 0 ]; then
    usage
    exit 1
fi

vms=`./lsvpc.py $VPC | grep -w VMInstance | cut -d " " -f 5`
vms_privaddr=`./lsvpc.py $VPC --privaddr | grep -w VMInstance | cut -d " " -f 6`

vma=($vms)
vma_privaddr=($vms_privaddr)
MAIN="${vma[0]}"
MAIN_PRIVADDR="${vma_privaddr[0]}"

run_scratch() {
  local vm=$1
  echo "VM: $vm"
  # No additional benchmarking setup needed for AWS.
  ssh -i key-$VPC.pem ubuntu@$vm /bin/bash <<ENDSSH
#    cd sigmaos
#    git fetch --all
#    git checkout osdi23-submit
#    git pull
#    ./make.sh --norace --version RETRY
#    ./install.sh --realm test-realm --version RETRY
#  sudo apt update
#  sudo apt install -y apparmor-utils
#  sudo aa-status
#  ls -lha /tmp/sigmaos-data/wiki-20G
#  sed -i "s|region=us-east-1|region = us-east-1|g" ~/.aws/credentials

#  sudo apt remove -y golang golang-go
#  wget https://go.dev/dl/go1.23.0.linux-amd64.tar.gz
#  sudo rm -rf /usr/local/go && sudo tar -C /usr/local -xzf go1.23.0.linux-amd64.tar.gz
#  sed -i.old '1s;^;export PATH=\$PATH:/usr/local/go/bin\n;' ~/.profile
#  sed -i.old '1s;^;export PATH=\$PATH:/usr/local/go/bin\n;' ~/.bashrc
#  go version
#echo "N Procq:"
#ps -fax | grep "bin/kernel/" | grep "/procq" | wc -l
#echo "N schedd:"
#ps -fax | grep "bin/kernel/" | grep "schedd" | wc -l
#sudo apt-get remove -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
#sudo apt-get install -y docker-ce=5:28.2.2-1~ubuntu.22.04~jammy docker-ce-cli=5:28.2.2-1~ubuntu.22.04~jammy containerd.io docker-buildx-plugin docker-compose-plugin
#printf "{\n\"exec-opts\": [\"native.cgroupdriver=systemd\"]\n}" | sudo tee /etc/docker/daemon.json
#sudo systemctl daemon-reload
#sudo systemctl restart docker
#sudo systemctl restart kubelet
#sudo containerd config default | sudo tee /etc/containerd/config.toml
#sudo sed -i 's/            SystemdCgroup = false/            SystemdCgroup = true/' /etc/containerd/config.toml
#sudo systemctl daemon-reload
#sudo systemctl restart docker
#sudo systemctl restart containerd
#sudo systemctl restart kubelet
#sudo systemctl restart containerd
#sudo groupadd docker
#sudo usermod -aG docker ubuntu
#sudo usermod -aG docker ubuntu
#docker --version
cd sigmaos
./set-cores.sh --start 4 --end 16 --set 0
nproc
docker pull arielszekely/sigmauser:arielck
ENDSSH
}

for vm in $vms; do
  if [ -n "$PARALLEL" ]; then
    run_scratch $vm &
  else
    run_scratch $vm
  fi
done
wait

#sudo kill -SIGTERM $(ps -fax | grep "argv0 /mnt/binfs/named" | cut -d " " -f3 | tail -n1)
#sudo kill -SIGTERM $(ps -fax | grep "argv0 named" | cut -d " " -f3 | tail -n1)


#ssh -i key-$VPC.pem ubuntu@$MAIN /bin/bash <<'ENDSSH'
#sudo kill -SIGTERM $(ps -fax | grep "argv0 /mnt/binfs/named" | cut -d " " -f3 | tail -n1)
#sudo kill -SIGTERM $(ps -fax | grep "argv0 named" | cut -d " " -f3 | tail -n1)
#ls -lha /tmp/sigmaos-perf
#ENDSSH
