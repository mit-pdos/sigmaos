# Creating/managing VPC

Run ./mkvpc.py to create a VPC, including one instance:
$ ./mkvpc.py ulam

If you specify, the vpc-id it will create a new instance:
$ ./mkvpc.py --vpc vpc-061a1808693a1626a ulam1

./lsvpc.py lists info about VPC:
$ ./lsvpc.py vpc-061a1808693a1626a

To install the ulambda software on the instance so that you can run it:
$ ./install-sw.sh key-vpc-061a1808693a1626a.pem ec2-52-90-134-108.compute-1.amazonaws.com

This clones the repo, and installs go. You should be able to run
./make.sh in the ulambda repo, and go test should succeed.

./lsrmvpc.py removes either an instance or the whole VPC
$ ./rmvpc.py  --vm i-04f877d38a65f1d05 vpc-061a1808693a1626a

# Running ulambda

To boot ulambda on the VPC:

$ ./start.sh vpc-061a1808693a1626a

will update the ulambda software on each EC2 instances and restart
ulambda daemons.

To login to the VPC:

$ ./login.sh vpc-061a1808693a1626a

this starts an ssh tunnel to the VPC. you only have to this once
(e.g., you can run ./start.sh again without having to login)

$ ./mount.sh mounts the VPC under /mnt/9p

$ ls /mnt/9p/

list the ulamba top-level directory on the VPC

$ sudo umount /mnt/9p

After umount you can run ./start.sh and ./mount.sh again to use the
updated lambda daemons on the VPC.

# AWS credentials

add keys for recipients to gpg key ring

$ gpg --recipient sigma-kaashoek --encrypt-files aws-credentials.txt
  
