#!/bin/bash

usage() {
  echo "Usage: $0 [--noreboot] --key VPC_KEY --vm VM_DNS_NAME" 1>&2
}

KEY=""
VM=""
REBOOT="reboot"
while [[ $# -gt 0 ]]
do
  case $1 in
  --noreboot)
    REBOOT="--noreboot"
    shift
    ;;
  --key)
    shift
    KEY=$1
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

echo $0 $REBOOT $KEY $VM

if [ -z "$KEY" ] || [ -z "$VM" ] || [ $# -gt 0 ]; then
  usage
  exit 1
fi

LOGIN=ubuntu
if [ $REBOOT = "reboot" ]; then
  # try to deal with lag before instance is created and configured
  echo -n "wait until cloud-init is done "
  
  while true; do
    done=`ssh -n -o ConnectionAttempts=1000 -i $KEY $LOGIN@$VM sudo grep "finished" /var/log/cloud-init-output.log`
    if [ ! -z "$done" ]; then
      break
    fi
    echo -n "."
    sleep 1
  done
  
  echo "done; reboot and wait"
  
  ssh -n -i $KEY $LOGIN@$VM sudo shutdown -r now
  
  sleep 2
  
  while true; do
    done=`ssh -n -i $KEY $LOGIN@$VM echo "this is an ssh"`
    if [ ! -z "$done" ]; then
      break
    fi
    echo -n "."
    sleep 1
  done
  
  echo "done rebooting"
fi

# Set up a few directories, and prepare to scp the aws secrets.
ssh -i $KEY $LOGIN@$VM <<ENDSSH
sudo mkdir -p /mnt/9p
mkdir ~/.aws
chmod 700 ~/.aws
echo > ~/.aws/credentials
chmod 600 ~/.aws/credentials
ENDSSH

# decrypt the aws secrets.
SECRETS=".aws/credentials"
for F in $SECRETS
do
  gpg --output $F --decrypt ${F}.gpg || exit 1
done

# scp the s3 secrets to the server and remove them locally.
scp -i $KEY .aws/config $LOGIN@$VM:/home/$LOGIN/.aws/
scp -i $KEY .aws/credentials $LOGIN@$VM:/home/$LOGIN/.aws/
rm $SECRETS

ssh -i $KEY $LOGIN@$VM <<ENDSSH
cat <<EOF > ~/.ssh/config
Host *
  StrictHostKeyChecking no
  UserKnownHostsFile=/dev/null
EOF
chmod go-w .ssh/config

cat << EOF > ~/.ssh/aws-ulambda
-----BEGIN OPENSSH PRIVATE KEY-----
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
-----END OPENSSH PRIVATE KEY-----
EOF
chmod 600 ~/.ssh/aws-ulambda

cat << EOF >> ~/.ssh/authorized_keys
ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQC4NF0v/XEFId9bJJ1KvzvIIfcFUPvvNJCWH35JJbpaCCRuguHAlim30WqeTG+Ru7Debl80AVuve+XrhL2uYY6R1SeBXQ6Vl6jGPzmmlTqJLi73e6oNWI13QJ1ALriS2Vy5xk1ckmS5epYS0OixerQJ/9gHTcdHWcNDbfUOi23jqdciNExSqjamrYvUwi14IhRNRqltrk2V4ephnRI+8S3ExansbZSwnu0XIz7j86e3PFMuuHwLJWv59UdO9roJl2B36dnzWp0lpqcXYrk3gbbXBCu6iV1Dv7XgvElTtmwqJJ50O2pzwJv2pBB/tw3LkWldF6FuYO3vjaTOgdm2gbCsw2DMJSa6oXJB4cRztXDe51ljbhdYptHxbJgM7+852soEma2uhuek80rRn3UEqrQ1MIsw0DJXx5k+tDbJAWyzy4k4opR583Go9UtRq/BY6qyaFHA/DY13c5QiJNapN5JameX3+wUvNmR22lX/SW61KFjXzYnn//77UCidNPr6SQs= kaashoek@fk6x1
EOF

if [ -d "ulambda" ] 
then
  ssh-agent bash -c 'ssh-add ~/.ssh/aws-ulambda; (cd ulambda; git pull;)'
else
  ssh-agent bash -c 'ssh-add ~/.ssh/aws-ulambda; git clone git@g.csail.mit.edu:ulambda; (cd ulambda; go mod download;)'
  # Indicate that sigma has not been build yet on this instance
  touch ~/.nobuild
fi

echo -n > ~/.hushlogin
ENDSSH

echo "== TO LOGIN TO VM INSTANCE USE: =="
echo "ssh -i $KEY $LOGIN@$VM"
echo "============================="

