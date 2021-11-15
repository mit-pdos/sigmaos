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

#ssh -i $DIR/keys/cloudlab-sigmaos $1 <<"ENDSSH" > $DIR/log/$LEADER
#    sudo kubeadm init --apiserver-advertise-address=10.10.1.1 --pod-network-cidr=10.10.1.0/24
# mkdir -p $HOME/.kube
# yes | sudo cp -i /etc/kubernetes/admin.conf $HOME/.kube/config
# sudo chown $(id -u):$(id -g) $HOME/.kube/config
# kubectl apply -f https://docs.projectcalico.org/v3.20/manifests/calico.yaml
# kubectl apply -f https://raw.githubusercontent.com/kubernetes/dashboard/v2.4.0/aio/deploy/recommended.yaml
# kubectl create serviceaccount --namespace kube-system tiller
# kubectl create clusterrolebinding tiller-cluster-rule --clusterrole=cluster-admin --serviceaccount=kube-system:tiller
# kubectl patch deploy --namespace kube-system tiller-deploy -p '{"spec":{"template":{"spec":{"serviceAccount":"tiller"}}}}'
#ENDSSH
echo "leader"

else

cmd="sudo $(tail -n 2 $DIR/log/$LEADER)"

ssh -i $DIR/keys/cloudlab-sigmaos $1 <<ENDSSH
  eval $cmd
ENDSSH

fi

ssh -i $DIR/keys/cloudlab-sigmaos $1 <<ENDSSH
ENDSSH
