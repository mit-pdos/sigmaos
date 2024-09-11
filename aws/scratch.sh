#!/bin/bash

usage() {
  echo "Usage: $0 --vpc VPC [--n N] [--taint N:M]" 1>&2
}

VPC=""
while [[ $# -gt 0 ]]; do
  key="$1"
  case $key in
  --vpc)
    shift
    VPC=$1
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

for vm in $vms; do
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
echo "N Procq:"
ps -fax | grep "bin/kernel/" | grep "/procq" | wc -l
echo "N schedd:"
ps -fax | grep "bin/kernel/" | grep "schedd" | wc -l
ENDSSH
done
