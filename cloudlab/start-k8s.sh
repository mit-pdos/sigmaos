#!/bin/bash

usage() {
  echo "Usage: $0 [--n N] [--taint N:M] [--swap]" 1>&2
}

N_VM=""
TAINT=""
SWAP=""
while [[ $# -gt 0 ]]; do
  key="$1"
  case $key in
  --vpc)
    shift
    shift
    ;;
  --taint)
    shift
    TAINT=$1
    shift
    ;;
  --n)
    shift
    N_VM=$1
    shift
    ;;
  --swap)
    shift
    SWAP="true"
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

if [ $# -gt 0 ]; then
    usage
    exit 1
fi

DIR=$(dirname $0)
source $DIR/env.sh

vms=`cat servers.txt | cut -d " " -f2` 

vma=($vms)
MAIN="${vma[0]}"
MAIN_PRIVADDR=$(./leader-ip.sh)

if ! [ -z "$N_VM" ]; then
  vms=${vma[@]:0:$N_VM}
fi

flannel_cidr="10.123.0.0"

join_cmd=""
kube_config=""

id=$(cat ~/.aws/credentials | grep "id" | tail -n 1 | cut -d ' ' -f3)
key=$(cat ~/.aws/credentials | grep "key" | tail -n 1 | cut -d ' ' -f3)

for vm in $vms; do
  $DIR/setup-for-benchmarking.sh $vm
  ssh -i $DIR/keys/cloudlab-sigmaos $LOGIN@$vm <<ENDSSH
  if [ "${vm}" = "${MAIN}" ]; then 
    echo "START k8s leader $vm"
    # Start the first k8s node.
    if [[ "${SWAP}" == "true" ]]; then
      echo "Swap is on, copying config"
      cp ~/sigmaos/aws/yaml/k8s-cluster-config-swap.yaml /tmp/kubelet.yaml
      sed -i "s/x.x.x.x/$MAIN_PRIVADDR/g" /tmp/kubelet.yaml
      sudo kubeadm init --config /tmp/kubelet.yaml 2>&1 | tee /tmp/start.out
    else
#      cp ~/sigmaos/cloudlab/yaml/k8s-cluster-config-epleg.yaml /tmp/kubelet.yaml
#      sed -i "s/x.x.x.x/$MAIN_PRIVADDR/g" /tmp/kubelet.yaml
#      sudo kubeadm init --config /tmp/kubelet.yaml 2>&1 | tee /tmp/start.out
      sudo kubeadm init --apiserver-advertise-address=$MAIN_PRIVADDR --pod-network-cidr=$flannel_cidr/16 2>&1 | tee /tmp/start.out
    fi
    mkdir -p ~/.kube
    yes | sudo cp -i /etc/kubernetes/admin.conf ~/.kube/config
    sudo chown $LOGIN ~/.kube/config

    # Install CNI
    rm /tmp/kube-flannel.yml
    wget -O /tmp/kube-flannel.yml https://github.com/flannel-io/flannel/releases/latest/download/kube-flannel.yml
    sed -i "s/10.244.0.0/$flannel_cidr/g" /tmp/kube-flannel.yml
    kubectl apply -f /tmp/kube-flannel.yml
    kubectl apply -f ~/sigmaos/benchmarks/k8s/metrics/metrics-server.yaml

    # Un-taint all nodes, so the control-plane node can run pods too
    kubectl taint nodes --all node-role.kubernetes.io/control-plane:NoSchedule-

    # Install dashboard
    kubectl apply -f https://raw.githubusercontent.com/kubernetes/dashboard/v2.4.0/aio/deploy/recommended.yaml
    kubectl create serviceaccount --namespace kubernetes-dashboard admin-user
#    kubectl create clusterrolebinding admin-user -p '{"spec":{"roleRef":{"spec":{"serviceAccount":"tiller"}}}}'

    # Create service account
#    kubectl create serviceaccount --namespace kube-system tiller
#    kubectl create clusterrolebinding tiller-cluster-rule --clusterrole=cluster-admin --serviceaccount=kube-system:tiller
#    kubectl patch deploy --namespace kube-system tiller-deploy -p '{"spec":{"template":{"spec":{"serviceAccount":"tiller"}}}}'

    # Register docker credentials
    kubectl create secret generic regcred --from-file=.dockerconfigjson=/users/$LOGIN/.docker/config.json  --type=kubernetes.io/dockerconfigjson

    # Register aws credentials
    kubectl create secret generic aws-creds --from-literal=aws-id=$id --from-literal=aws-secret=$key
  else
    echo "JOIN k8s follower $vm"
    if [ -z "$join_cmd" ]; then
        echo "No join command specified"
        exit 1
    fi
    eval "sudo $join_cmd"
    mkdir -p ~/.kube
    echo "$kube_config" > ~/.kube/config
    sudo chown $LOGIN ~/.kube/config
  fi
ENDSSH
  # Get command for follower nodes to join the cluster
  if [ "${vm}" = "${MAIN}" ]; then
    print_join_cmd="
      ssh -i $DIR/keys/cloudlab-sigmaos $LOGIN@$vm <<ENDSSH
        kubeadm token create --print-join-command
ENDSSH"
    print_kube_config="
      ssh -i $DIR/keys/cloudlab-sigmaos $LOGIN@$vm <<ENDSSH
        cat ~/.kube/config
ENDSSH"
    join_cmd=$(eval "$print_join_cmd")
    kube_config=$(eval "$print_kube_config")
  fi
done

# If desired, taint benchmark driver nodes.
if ! [ -z "$TAINT" ]; then
  x1=$(echo $TAINT | cut -d ":" -f1)
  x2=$(echo $TAINT | cut -d ":" -f2)
  to_taint="${vma[@]:$x1:$x2}"
  to_taint_a=$(echo $to_taint)
  for vm in $to_taint_a; do
    nodename=$(ssh -i $DIR/keys/cloudlab-sigmaos $LOGIN@$vm hostname)
    ssh -i $DIR/keys/cloudlab-sigmaos $LOGIN@$vm <<ENDSSH
        echo "Tainting node $nodename"
        kubectl taint nodes $nodename t=benchdriver:NoSchedule
ENDSSH
  done
fi
