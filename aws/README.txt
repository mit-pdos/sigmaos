# Creating/managing VPC

Run ./mkvpc.py to create a VPC, including one instance:
$ ./mkvpc.py ulam

If you specify, the vpc-id it will create a new instance:
$ ./mkvpc.py --vpc vpc-061a1808693a1626a ulam1

./lsvpc.py lists info about VPC:
$ ./lsvpc.py vpc-061a1808693a1626a

To download the sigmaos software on an instance the first time it is being set up:
$ ./setup-instance.sh --vpc vpc-061a1808693a1626a --vm ec2-52-90-134-108.compute-1.amazonaws.com

To build the sigmaos software and upload the build for all instances to pull:
$ ./build-sigma.sh --vpc vpc-061a1808693a1626a --realm fkaashoek

To install the latest version of the sigmaos kernel on all instances:
$ ./install-sigma.sh --vpc vpc-061a1808693a1626a --realm fkaashoek

./rmvpc.py removes either an instance or the whole VPC
$ ./rmvpc.py --vm i-04f877d38a65f1d05 vpc-061a1808693a1626a

# Running sigmaos 

To boot sigmaos on the VPC:

$ ./start-sigmaos.sh --vpc vpc-061a1808693a1626a --realm fkaashoek

will update the sigmaos software on each EC2 instances and restart
sigmaos daemons.

To login to the VPC:

$ ./login.sh --vpc vpc-061a1808693a1626a

this starts an ssh tunnel to the VPC. you only have to this once
(e.g., you can run ./start.sh again without having to login)

Set the NAMED environment variable, using the machine's full internal ip as so:

$ export NAMED="10.x.x.x:1111"

To mount the VPC under /mnt/9p:
$ ./mount.sh 

To list the ulamba top-level directory on the VPC:
$ ls /mnt/9p/

To unmount the VPC:
$ sudo umount /mnt/9p

After umount you can run ./start.sh and ./mount.sh again to use the
updated lambda daemons on the VPC.

# AWS credentials

add keys for recipients to gpg key ring

$ gpg --recipient sigma-kaashoek --recipient arielck --recipient NEW_RECIPIENT --encrypt-files aws-credentials.txt
