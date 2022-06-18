# Building, installing, and running sigmaos.

To build sigmaos (and generate a new build version), run:
$ ./make.sh 

To upload the built binaries to an s3 bucket corresponding to realm REALM, run
(optional, purely local development also possible as described below):
$ ./upload.sh --realm REALM

To install the sigmaos kernel (realm and kernel packages) for realm REALM, run:
$ ./install.sh --realm REALM --from s3

If running without internet connectivity, everything can be installed locally
by running:
$ ./install.sh --realm REALM --from local

To garbage-collect old build versions from s3 for realm REALM:
$ ./rm-old-versions-s3.sh --realm REALM

To start sigmaos (having already run make.sh, optionally upload.sh, and then
install.sh), and create a realm REALM run:
$ ./start.sh --realm REALM

To stop sigmaos, run:
$ ./stop.sh
