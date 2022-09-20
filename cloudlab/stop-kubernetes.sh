#!/bin/bash

if [ "$#" -ne 1 ]
then
  echo "Usage: $0 user@address"
  exit 1
fi

DIR=$(dirname $0)

. $DIR/config

is_leader=0
address=$(echo $1 | cut -d "@" -f2)

while read -r line
do
  tuple=($line)

  hostname=${tuple[0]}
  addr=${tuple[1]}

  if [[ $hostname == $LEADER ]]; then
    if [[ $address == $addr ]]; then
      is_leader=1
    fi
  fi
done < $DIR/$SERVERS

if [[ $is_leader == 1 ]]; then

ssh -i $DIR/keys/cloudlab-sigmaos $1 <<"ENDSSH"
#  #kubectl delete -f ~/sigmaos/cloudlab/k8s/cni/calico.yaml
#  #kubectl delete -f ~/sigmaos/cloudlab/k8s/cni/tigera-operator.yaml
#  # Get node names
#  lines=$(kubectl get nodes | tail -n +2)
#  while IFS= read -r line; do
#    name=$(echo $line | cut -d " " -f1)
#    kubectl drain $name --delete-emptydir-data --force --ignore-daemonsets
#    kubectl delete node $name
#  done <<< "$lines"
  yes | sudo kubeadm reset
ENDSSH

else

ssh -i $DIR/keys/cloudlab-sigmaos $1 <<"ENDSSH"
  yes | sudo kubeadm reset
ENDSSH

fi
