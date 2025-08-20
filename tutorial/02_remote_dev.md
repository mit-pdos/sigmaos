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
infrastructure downloads the binaries from at runtime. The build scripts expect
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

The remainder of this tutorial will refer to your tag as `TAG`. When running
the scripts, make sure to replace `TAG` with your own tag name.  The `--target`
argument will be `remote` regardless of deployment platform (CloudLab or EC2).

Build the SigmaOS images for remote deployment and push user `proc` binaries to
S3 with:

```
$ ./build.sh --parallel --target remote --push TAG
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

The SigmaOS CloudLab experiments are all run on c220g5 machines running Ubuntu
24.04, and thus some scripts (such as the package install scripts and disk
partition setup) may not run correctly without modification on other CloudLab
instance types. Proceed with caution if using another instance type.

First, set the `LOGIN` variable in `cloudlab/env.sh` to your cloudlab username,
USERNAME.

Then, go to the CloudLab experiment manifest page, and copy the XML-formatted
description of your cloudlab cluster into a local file (e.g.,
`/tmp/cloudlab-servers.txt`). Then, from the `sigmaos/cloudlab` directory, run
the manifest ingestion script to store a description of the clusters in a
format usable by other SigmaOS CloudLab scripts (supplying your CloudLab
username in place of `USERNAME`):

```
$ ./import-server-manifest.sh --manifest /tmp/servers.txt --username USERNAME 
```

Next, make sure that you can decrypt the DockerHub/AWS credentials checked into
the repo by running the following line from the root of the SigmaOS repo, e.g.:

```
$ gpg --output /tmp/xxxxx --decrypt aws/.aws/credentials.gpg; rm /tmp/xxxxx
```

Then, you can set up the CloudLab cluster by running one script. From the root
of the SigmaOS repo, run:

```
$ cd cloudlab
$ ./setup-cluster.sh
```

This should install SigmaOS and all of its dependencies, and configure the OS
kernel on each machine to prepare the machines for benchmarking.Internally, the
`setup-cluster.sh` script runs two scripts for each machine: one script which
installs SigmaOS and its dependencies, and another that configures the OS
kernel. The `setup-cluster.sh` script will pipe output from each script into
its own file, for each machine in the cluster. Output from
the scripts is stored in `/tmp/sigmaos-cloudlab-node-logs`. This output can be
helpful in the event that one of the scripts doesn't run to completion
correctly. Feel free to skip the description of the CloudLab setup scripts
below (unless one of them produced an error).

If the install scripts ran successfully, you should see all machines in your
cluster running, and each should have the SigmaOS repo in `$HOME`. To verify
your installation, build the SigmaOS binaries for remote development, and then
run the following scripts and ensure that they terminatea successfully:

```
$ ./stop-sigmaos.sh --vpc $VPC --parallel
$ ./start-sigmaos.sh --vpc $VPC --pull TAG --branch BRANCH
```

These scripts should take no more than a couple of minutes to run. If they
hang, please reach out for help :) .

For the remainder of this tutorial,
replace USER and HOSTNAME with your username and the DNS name of the machine
you wish to run the script on.

##### configure-kernel.sh

CloudLab sets up its machines with a very small root partition (usually 15G)
and a large, unmounted partition. This causes problems for both SigmaOS and
Kubernetes, since Docker and Kubernetes store container logs, images, and other
data in the root partition by default, which fills up quickly.  Additionally,
CloudLab's default kernel configuration runs with CPU frequency scaling and
c-states on, which can cause performance variablity during benchmarking, and
runs with cgroupsv1 by default (whereas SigmaOS requires cgroupsv2).

The `./configure-kernel.sh` script takes care of configuring the kernel,
formatting, and mounting a large data volume for SigmaOS, Kubernetes, and
Docker to use. Make sure that each machine successfully restarts after running
the kernel configuration script on it.

### Updating SigmaOS

After building a new version of the SigmaOS containers and binaries and
restarting the cluster, the SigmaOS software and scripts should take care of
updating the cluster to the newest version.  However, changes to the SigmaOS
git repo will not be automatically reflected on remote machines. This is
particularly relevant to benchmarks, which are implemented as `go` files
included in the repo. 

In order to update the repo on a remote cluster, run (you only need VPCID for
AWS): 

```
$ cd PLATFORM
$ ./update-repo.sh USER --vpc VPCID --parallel
```

If you wish to switch to a different branch `BRANCH` before pulling, run:

```
$ cd PLATFORM
$ ./update-repo.sh USER --vpc VPCID --parallel --branch BRANCH
```

### Deploying SigmaOS

In order to start a SigmaOS cluster, run:

```
$ cd PLATFORM
$ ./start-sigmaos.sh USER --vpc VPCID --pull TAG
```

If you wish to only start SigmaOS on a subset `N` of the machines in the
cluster, run:

```
$ cd PLATFORM
$ ./start-sigmaos.sh USER --vpc VPCID --pull TAG --n N
```

To verify SigmaOS status on each machine, run

```
for h in $(cat servers.txt | cut -d " " -f 2); do echo $h; ssh USER@$h "docker ps -a"; done
```

Each node should be running an instance of the sigmaos container. Node 0, should
additionally have the mariadb container. 

In order to stop the SigmaOS deployment, run:

```
$ cd PLATFORM
$ ./stop-sigmaos.sh USER --vpc VPCID --parallel
```

## Quirks

Both AWS and CloudLab have some quirks which are important to know about when
deploying SigmaOS. This section describes some of them.

### CloudLab Quirks

I have run into quite a few quirks while working with CloudLab. Their users
Google group is very active and useful, and this is the best way to reach the
support team, in my experience. You can access it
[here](https://groups.google.com/g/cloudlab-users).

Some specific examples of issues I've run into (and solutions) can be found
here:

- Some of the CloudLab hardware is faulty. In particular, I've had issues where
  the top-of-rack network switch sometimes fails unexpectedly, and some
  machines are unable to talk to each other for a short time while the switch
  reboots. Other lab members (e.g., Zain), have run into bad NICs which
  sometimes drop packets of fail entirely. The best thing to do is run a "Link
  Test" as soon as you get your cluster, to make sure the network is fully
  functional. You can start a link test from the home page of your experiment.
- CloudLab sets up its machines to have a very small root partition, which
  fills up easily. Any data or logging should make sure to be written to a
  larger partition.
- Many CloudLab machines have multiple CPUs/NUMA nodes. However, the Linux
  Scheduler has bugs which cause unexpected performance issues which manifest
  in multi-NUMA node configurations. As such, it's important to turn off the
  second CPU when benchmarking.
- CloudLab machines have many network interfaces. In order to avoid being
  throttled, make sure to always use the local cluster IP range, 10.10.0.0/16.

### AWS Quirks

- AWS VMs are hyperthreaded, which may increase interference between threads on
  the same machine.
- In my experience, it seems that AWS VM and network performance fluctuates
  slightly with the time of day.
