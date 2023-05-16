# 02. Remote deployment

This tutorial describes how to build SigmaOS for remote deployment and
benchmarking, and deploy SigmaOS on a remote set of machines. By the end of
this tutorial, you should be able to start a SigmaOS cluster on a remote set of
machines, and run benchmarks on the cluster.

## Platforms

Currently, we deploy SigmaOS in two remote settings:
[CloudLab](https://cloudlab.us/) and [AWS EC2](https://aws.amazon.com/ec2/).

CloudLab is an NSF-funded data center ecosystem which is free to use for
researchers, and gives researchers access to bare-metal hardware. This makes
CloudLab useful for development and well as benchmarking latency-sensitive
applications which are sensitive to the performance overheads caused by
virtualization and a shared datacenter network. AWS EC2 is useful for
benchmarks which need fast access to S3, which is SigmaOS's long-term stable
storage backend.

Deploying and running SigmaOS on CloudLab and AWS is intentionally similar. The
scripts in the `cloudlab` and `aws` directories should match in name,
arguments, and functionality, which should (hopefully) make switching between
the two seamless. However, each platform has its quirks, and may require
additional setup. The following sections describe the generic build and
deployment process (applicable to both CloudLab and AWS), as well as the
platform-specific quirks and installation instructions.

## Building SigmaOS for remote deployment

When benchmarking and remotely deploying SigmaOS, we build a version of the
SigmaOS container images which excludes user-level `proc` binaries.  Instead,
the binaries are uploaded and stored in an S3 bucket, which the SigmaOS
infrastructure downlaods the binaries from at runtime. The build scripts expect
the S3 bucket to already exist and for the AWS and DockerHub credentials to be
installed on your computer, so make sure to poke a member of the existing
development team and tell them to complete their side of the [onboarding
tasks](./onboarding.md) for you before continuing.

Install the Docker and AWS credentials on your computer with:

```
./install-cred.sh
```

When building SigmaOS for remote deployment, we build once locally and upload
the result. SigmaOS cluster then transparently downloads the latest build
version. In order to build SigmaOS for remote deployment, you will need to use
a "tag" which, tells the SigmaOS build and deployment scripts which version of
the SigmaOS container images to upload and download, as well as the name of the
S3 bucket in which the user `proc` binaries will be stored. For now, the "tag"
should be the same as the name of the S3 bucket which the SigmaOS development
team created for you during [onboarding](./onboarding.md).

For the remainder of this tutorial, I will refer to your tag as `TAG`. When
running the scripts, make sure to replace `TAG` with your own tag name.
However, the `--target` argument will be `aws` regardless of deployment
platform (CloudLab or EC2).

Build the SigmaOS images for remote deployment and push user `proc` binaries to
S3 with:

```
$ ./build.sh --parallel --target aws --push TAG
```

## Installing, updating, and deploying SigmaOS on a remote cluster

The following scripts need to be run from the directory corresponding to the
deployment platform: either `cloudlab` or `aws`. Make sure to replace
`PLATFORM` with your deployment platform where appropriate.  Additionally, the
AWS scripts expect a `--vpc` argument. Make sure to replace the following VPCID
with your VPC's ID. The CloudLab scripts accept the `--vpc` argument for the
sake of uniformity, but ignore it. Feel free to omit it when running CloudLab
scripts.

In the remainder of this section, each sequence of commands assumes that the
current working directory is the root of the project repo.

### Installing SigmaOS

Installing SigmaOS differs slightly depending on the platform you are running
on. Below are the installation instructions for each platform.

#### AWS.

XXX TODO.

#### CloudLab.

CloudLab runs an old version of the Linux Kernel, so you need to upgrade the
kernel before you can install SigmaOS. Also, CloudLab sets up its machines with
a very small root partition (usually 15G) and a large, unmounted partition. In
order to run SigmaOS and install the new kernel (both of which can use a
significant amount of disk space), you will need to mount and format the new
disk partition. The `./upgrade-linux.sh` script takes care of both setting up
the new partition and installing the new version of the kernel.

For the remainder of this section,
replace USER and HOSTNAME with your username and the DNS name of the machine
you wish to run the script on.

First, find the name of the large, unused partition on the cloudlab machines
you are using by logging into one of them and running:

```
$ lsblk
```

Then, run the `cloudlab/upgrade-linux.sh` with your cloudlab username, machine 
name, and partition block name. For example, on `c220g5` machines, this is `/dev/sda4`.

Then, upgrade the Linux Kernel on each machine by running:

```
$ cd cloudlab
$ ./upgrade-linux.sh USER HOSTNAME BLKDEV
```

If you are setting up a multi-machine clutser, it may be convenient to run this
script in parallel in a bash for loop, like so:

```
$ cd cloudlab
$ for h in $(cat servers.txt | cut -d " " -f 2); do
echo "=========== Upgrading linux for $h";
./upgrade-linux.sh arielck $h /dev/sda4 >& /tmp/$h.out;
done
```

Note: for some reason, this doesn't always work on the first try. You may need
to try to install the kernel twice, by rerunning the `upgrade-linux.sh` script.

Then, install the SigmaOS software, credentials, and its dependencies by
running:

```
$ cd cloudlab
$ ./setup-instance.sh USER@HOSTNAME
```

### Updating SigmaOS

After building a new version of the SigmaOS containers and binaries and
restarting the cluster, the SigmaOS software and scripts should take care of
updating the cluster to the newest version.  However, changes to the SigmaOS
git repo will not be automatically reflected on remote machines. This is
particularly relevant to benchmarks, which are implemented as `go` files
included in the repo.

In order to update the repo on a remote cluster, run:

```
$ cd PLATFORM
$ ./update-repo.sh --vpc VPCID --parallel
```

If you wish to switch to a different branch `BRANCH` before pulling, run:

```
$ cd PLATFORM
$ ./update-repo.sh --vpc VPCID --parallel --branch BRANCH
```

### Deploying SigmaOS

In order to start a SigmaOS cluster, run:

```
$ cd PLATFORM
$ ./start-sigmaos.sh --vpc VPCID --pull TAG
```

If you wish to only start SigmaOS on a subset `N` of the machines in the
cluster, run:

```
$ cd PLATFORM
$ ./start-sigmaos.sh --vpc VPCID --pull TAG --n N
```

In order to stop the SigmaOS deployment, run:

```
$ cd PLATFORM
$ ./stop-sigmaos.sh --vpc VPCID --parallel
```
