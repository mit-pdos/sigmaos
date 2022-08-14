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

ssh -i $DIR/keys/cloudlab-sigmaos $1 <<"ENDSSH" > $DIR/log/$LEADER
 sudo kubeadm init --apiserver-advertise-address=10.10.1.1 --pod-network-cidr=11.0.0.0/16
 mkdir -p $HOME/.kube
 yes | sudo cp -i /etc/kubernetes/admin.conf $HOME/.kube/config
 sudo chown $(id -u):$(id -g) $HOME/.kube/config
 # Install CNI
 kubectl create -f ~/ulambda/cloudlab/k8s/cni/tigera-operator.yaml
 kubectl create -f ~/ulambda/cloudlab/k8s/cni/calico.yaml
 kubectl create -f ~/ulambda/cloudlab/k8s/metrics/metrics-server.yaml
 kubectl apply -f https://raw.githubusercontent.com/kubernetes/dashboard/v2.4.0/aio/deploy/recommended.yaml
# kubectl apply -f https://github.com/kubernetes-sigs/metrics-server/releases/latest/download/components.yaml
 kubectl create serviceaccount --namespace kube-system tiller
 kubectl create clusterrolebinding tiller-cluster-rule --clusterrole=cluster-admin --serviceaccount=kube-system:tiller
 kubectl patch deploy --namespace kube-system tiller-deploy -p '{"spec":{"template":{"spec":{"serviceAccount":"tiller"}}}}'
 sudo kubectl create secret generic regcred --from-file=.dockerconfigjson=/users/arielck/.docker/config.json  --type=kubernetes.io/dockerconfigjson
ENDSSH

echo "leader"

else

cmd="sudo $(grep -v 'created'  $DIR/log/$LEADER | tail -n 2)"

ssh -i $DIR/keys/cloudlab-sigmaos $1 <<ENDSSH
  eval $cmd
ENDSSH

fi

ssh -i $DIR/keys/cloudlab-sigmaos $1 <<ENDSSH
ENDSSH
