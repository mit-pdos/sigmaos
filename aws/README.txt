Run ./mkvpc.py to create a VPC, including one instance:
$ ./mkvpc.py ulam

If you specify, the vpc-id it will create a new instance:
$ ./mkvpc.py --vpc vpc-061a1808693a1626a ulam

./lsvpc.py lists info about VPC:
$ ./lsvpc.py vpc-061a1808693a1626a

To install the ulambda software on the instance so that you can run it:
$ ./install-sw.sh key-vpc-061a1808693a1626a.pem ec2-52-90-134-108.compute-1.amazonaws.com

This clones the repo, and installs go. You should be able to run
./make.sh in the ulambda repo, and go test should succeed.

./lsrmvpc.py removes either an instance or the whole VPC
$ ./rmvpc.py  --vm i-04f877d38a65f1d05 vpc-061a1808693a1626a
