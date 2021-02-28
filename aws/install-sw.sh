#!/bin/sh

if [ "$#" -ne 2 ]
then
  echo "Usage: ./install-sw.sh vpc-key-file dns-instance"
  exit 1
fi

echo "$0 $1 $2"

# try to deal with lag before instance is created and configured
echo -n "wait until cloud-init is done "

while true ; do
    done=`ssh -n -o ConnectionAttempts=1000 -i $1 ubuntu@$2 grep "finished" /var/log/cloud-init-output.log`
    if [ ! -z "$done" ]
    then
	break
    fi
    echo -n "."
    sleep 1
done

echo "done; reboot and wait"

ssh -n -i $1 ubuntu@$2 sudo shutdown -r now

sleep 2

while true ; do
    done=`ssh -n -i $1 ubuntu@$2 echo "this is an ssh"`
    if [ ! -z "$done" ]
    then
	break
    fi
    echo -n "."
    sleep 1
done

echo "done rebooting"

ssh -n -i $1 ubuntu@$2 sudo snap install --classic go

ssh -i $1 ubuntu@$2 <<ENDSSH
cat <<EOF > ~/.ssh/config
Host *
   StrictHostKeyChecking no
   UserKnownHostsFile=/dev/null
EOF
cat << EOF > ~/.ssh/aws-ulambda
-----BEGIN OPENSSH PRIVATE KEY-----
b3BlbnNzaC1rZXktdjEAAAAABG5vbmUAAAAEbm9uZQAAAAAAAAABAAABlwAAAAdzc2gtcn
NhAAAAAwEAAQAAAYEAuDRdL/1xBSHfWySdSr87yCH3BVD77zSQlh9+SSW6WggkboLhwJYp
t9Fqnkxvkbuw3m5fNAFbr3vl64S9rmGOkdUngV0OlZeoxj85ppU6iS4u93uqDViNd0CdQC
64ktlcucZNXJJkuXqWEtDosXq0Cf/YB03HR1nDQ231Dott46nXIjRMUqo2pq2L1MIteCIU
TUapba5NleHqYZ0SPvEtxMWp7G2UsJ7tFyM+4/OntzxTLrh8CyVr+fVHTva6CZdgd+nZ81
qdJaanF2K5N4G21wQruoldQ7+14LxJU7ZsKiSedDtqc8Cb9qQQf7cNy5FpXRehbmDt742k
zoHZtoGwrMNgzCUmuqFyQeHEc7Vw3udZY24XWKbR8WyYDO/vOdrKBJmtrobnpPNK0Z91BK
q0NTCLMNAyV8eZPrQ2yQFss8uJOKKUefNxqPVLUavwWOqsmhRwPw2Nd3OUIiTWqTeSWpnl
9/sFLzZkdtpV/0lutShY182J5//++1AonTT6+kkLAAAFiLeA8nm3gPJ5AAAAB3NzaC1yc2
EAAAGBALg0XS/9cQUh31sknUq/O8gh9wVQ++80kJYffkkluloIJG6C4cCWKbfRap5Mb5G7
sN5uXzQBW6975euEva5hjpHVJ4FdDpWXqMY/OaaVOokuLvd7qg1YjXdAnUAuuJLZXLnGTV
ySZLl6lhLQ6LF6tAn/2AdNx0dZw0Nt9Q6LbeOp1yI0TFKqNqati9TCLXgiFE1GqW2uTZXh
6mGdEj7xLcTFqextlLCe7RcjPuPzp7c8Uy64fAsla/n1R072ugmXYHfp2fNanSWmpxdiuT
eBttcEK7qJXUO/teC8SVO2bCoknnQ7anPAm/akEH+3DcuRaV0XoW5g7e+NpM6B2baBsKzD
YMwlJrqhckHhxHO1cN7nWWNuF1im0fFsmAzv7znaygSZra6G56TzStGfdQSqtDUwizDQMl
fHmT60NskBbLPLiTiilHnzcaj1S1Gr8FjqrJoUcD8NjXdzlCIk1qk3klqZ5ff7BS82ZHba
Vf9JbrUoWNfNief//vtQKJ00+vpJCwAAAAMBAAEAAAGAQzh4uUaXs/PEepJGBDz9J++dIY
eIKKbaDK65eyAVfT1zbhC9KXajCr2VVuc138zKoqFMhMHqH4PrR6NNRRG84WMT45V9+QE7
Sf1QoIXcsqrz4MR0LiKenWIrAH0MRiqkN/cxzBJwR2+AzKUIBZxtjyiaQTAVBOiw5S5/Oy
uOkoIsTAMhxxjukKzMVnv1890b44dh1PDCx2uGCNE1E1sYBLLNa8YheMLGaqcmEDGehrOw
91/DhJ3HZZQozZwkD9P5dcke3NBB0zScCWufKtGXstvRUkrpNiHMxstk0D+6m7+4goA7sT
sydwkwPQaH3+irvWpZMWVGyJjsaCTKoqgYyycgxztgpF70usgeisDXKeh/1ybZ9VZDDESD
RoYrJdKnsGm0r0byTRlRm+EwdWQo+lwhDAJOHy5zeEhW9CW6SNlr8sVOcZXPgFaOz279Ib
16YLanbS/qGkGfH4LfB3IpXm27z13PgdaiYpZCnJpNpOMzThtbfxlKE04qU+hglq4BAAAA
wDwpXrSNUnCTw7CkERGrZiRPhg27p5hdKNH/KvQ+DwpPuq2I+mwLLIl8ukdXaQMPrW7p/7
DhECCxZVSSzn4TNKVeuqC0fEm2QIh9O1/sXeykeMlSEeK5p8jWhRKEFK16pIXYZNKMS/ZP
7q5QK3GNBuUpXSrTFSIlNTu2Gann0Id4NpxQLN23LUGEJoidgmwvY82trtKtIy5eqH1NB7
Joiwjmiatv8e0yE3r31yUXPSiGtX1AI15YH4hMVtSLRoxCnAAAAMEA20EHsAPSr+CgF916
k23Oi8/IWoRp1pBLGjETkl9+tPWGhStKvLRwMQsFqw6qMVza6A9Osc43813nHYGiDk7BPf
ZWOi5Cw9G8rt10DZOHHHTBGz2GiSSbztyq4Y1Bb2xOP93+kJ1Wei60puNyeZsi6iYVxGVF
7grIKi6Jsfj55+soXGY4t999auEfCeQWafhjesKLtt3en4sl1ZBIJ+Bgvwt/fTZafAF+9F
Rq6wc8w59/m5r0OkLZtl6OopqymvRBAAAAwQDXE45V2F5EuPn1zgbINO9Pk70Yjlq7Swuh
gXO27ekugwLYJe/UmlM5jHZGxuhZrMn+G9HRnMVWD78M1FfyU/PkzdsEwPxfSbl+q3+YRv
jCPLaDog91uFPSlez3OC/eEKgCCA6WUP6w9X80VpLvi2kPumXsJXPIcQvAmpQqYPeK+ELt
8slhWko85pUd4wijbtZvrOtMdtoFo5Eut0DwkLAJ3HlDpWFGAGasmte8/RulDiJlBByRNv
IuzhbPSUS+OksAAAAOa2Fhc2hvZWtAZms2eDEBAgMEBQ==
-----END OPENSSH PRIVATE KEY-----
EOF
chmod 600 ~/.ssh/aws-ulambda
if [ -d "ulambda" ] 
then
   ssh-agent bash -c 'ssh-add ~/.ssh/aws-ulambda; (cd ulambda; git pull)'
else
   ssh-agent bash -c 'ssh-add ~/.ssh/aws-ulambda; git clone git@g.csail.mit.edu:ulambda'
fi
ENDSSH

echo "== TO LOGIN TO VM INSTANCE USE: =="
echo "ssh -i $1 ubuntu@$2"
echo "============================="

