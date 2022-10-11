#!/bin/bash

usage() {
  echo "Usage: $0 --vpc VPC [--n N] " 1>&2
}

N_VM=""
VPC=""
while [[ $# -gt 0 ]]; do
  key="$1"
  case $key in
  --vpc)
    shift
    VPC=$1
    shift
    ;;
  --n)
    shift
    N_VM=$1
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

DIR=$(dirname $0)
. $DIR/../.env

vms=`./lsvpc.py $VPC | grep -w VMInstance | cut -d " " -f 5`
vms_privaddr=`./lsvpc.py $VPC --privaddr | grep -w VMInstance | cut -d " " -f 6`

vma=($vms)
vma_privaddr=($vms_privaddr)
MAIN="${vma[0]}"
MAIN_PRIVADDR="${vma_privaddr[0]}"

if ! [ -z "$N_VM" ]; then
  vms=${vma[@]:0:$N_VM}
fi

join_cmd=""

for vm in $vms; do
  ssh -i key-$VPC.pem ubuntu@$vm /bin/bash <<ENDSSH
  if [ "${vm}" = "${MAIN}" ]; then 
    echo "START k8s leader $vm"
    # Start the first k8s node.
    sudo kubeadm init --apiserver-advertise-address=$MAIN_PRIVADDR --pod-network-cidr=192.168.0.0/16 2>&1 | tee /tmp/start.out
    mkdir -p ~/.kube
    yes | sudo cp -i /etc/kubernetes/admin.conf ~/.kube/config
    sudo chown 1000:1000 ~/.kube/config

    # Install CNI
    kubectl create -f https://raw.githubusercontent.com/projectcalico/calico/v3.24.1/manifests/tigera-operator.yaml
    kubectl create -f https://raw.githubusercontent.com/projectcalico/calico/v3.24.1/manifests/custom-resources.yaml
    kubectl create -f ~/ulambda/benchmarks/k8s/metrics/metrics-server.yaml

    # Un-taint all nodes, so the control-plane node can run pods too
    kubectl taint nodes --all node-role.kubernetes.io/control-plane-

    # Install dashboard
    kubectl apply -f https://raw.githubusercontent.com/kubernetes/dashboard/v2.4.0/aio/deploy/recommended.yaml

    # Create service account
    kubectl create serviceaccount --namespace kube-system tiller
    kubectl create clusterrolebinding tiller-cluster-rule --clusterrole=cluster-admin --serviceaccount=kube-system:tiller
    kubectl patch deploy --namespace kube-system tiller-deploy -p '{"spec":{"template":{"spec":{"serviceAccount":"tiller"}}}}'

    # Register docker credentials
    kubectl create secret generic regcred --from-file=.dockerconfigjson=/home/ubuntu/.docker/config.json  --type=kubernetes.io/dockerconfigjson
  else
    echo "JOIN k8s follower $vm"
    if [ -z "$join_cmd" ]; then
        echo "No join command specified"
        exit 1
    fi
    eval "sudo $join_cmd"
  fi
ENDSSH
  # Get command for follower nodes to join the cluster
  if [ "${vm}" = "${MAIN}" ]; then
    print_join_cmd="
      ssh -i key-$VPC.pem ubuntu@$vm /bin/bash <<ENDSSH
        kubeadm token create --print-join-command
ENDSSH"
    join_cmd=$(eval "$print_join_cmd")
  fi
done
