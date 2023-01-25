# Building, installing, and running sigmaos.

Make sure docker is installed.

To augment PATH to find proxyd and, if desired, set SIGMADEBUG
$ source env/env.sh

So that sigmaos can access host's docker.sock within container:
# chmod 666 /var/run/docker.sock

This setup is insecure but see comments in bootkernelclnt.
Small bonus we don't have to run tests etc. as root:

Build docker images (one for kernel and one for user)
$ ./build.sh

To run tests for package PACKAGE_NAME, run:
$ go test -v sigmaos/PACKAGE_NAME

To upload the built binaries to an s3 bucket corresponding to realm REALM, run
(optional, purely local development also possible as described below):
$ ./upload.sh --realm REALM

To stop proxy (e.g., if test fails), run:
$ ./stop.sh

============================================================

XXX need to update

Full build flow for 4 development modes, first set the realm name in an
environment variable by running:
$ export REALM_NAME=fkaashoek

Then, run one of the 4 options below:

1. When developing locally and testing locally without internet access (no
access to s3), run:
$ ./make.sh --norace && ./install.sh --realm $REALM_NAME --from local

2. When developing locally and testing locally with internet access (pushing
and pulling from s3) and without garbage collecting old binary versions stored
in s3, run:
$ ./make.sh --norace && ./upload.sh --realm $REALM_NAME && ./install.sh --realm $REALM_NAME --from s3

3. When developing locally and testing locally with internet access (pushing
and pulling from s3), and garbage collecting old binary versions stored in s3,
run:
$ ./make.sh --norace && ./upload.sh --realm $REALM_NAME && ./install.sh --realm $REALM_NAME --from s3 && ./rm-old-versions-s3.sh --realm $REALM_NAME

4. When developing locally and testing on ec2, refer to "aws/README.txt" to
build the binaries on an ec2 instance. Then, run tests as described above.
