# SigmaOS SOSP24 Artifact

This README describes how to reproduce the results in our SOSP24 paper.

## Resources for reviewers

For the purpose of artifact the evaluation, we will provide two small clusters
of AWS machines (and provide setup scripts to set up a
[CloudLab](https://www.cloudlab.us/) cluster which can be procured by the
artifact reviewers). Since multiple reviewers may share the clusters, we ask
that reviewers coordinate to conduct their reviews at different times.

### Unpackaging the reviewer's artifact package

Please download the `.tar.gz` file, and untar it with:

```
$ tar -xzf sigmaos-artifact-pkg.tar.gz
```

This package contains keys which will be necessary to access the AWS clusters
we set up for the reviewers, as well as AWS credentials. They are useful for
setting up a CloudLab cluster using our scripts.

### Accessing provided AWS clusters (for reviewers)

The artifact submission package contains VPC keys required to access both AWS
clusters, and the driver machine (which is used to run the experiments and
produce the graphs, and resides in the first cluster), also has these keys
available.

The driver machine can be accessed by running the following:

```
$ cd sigmaos-artifact-pkg
$ ssh -i key-vpc-0affa7f07bd923811.pem ubuntu@ec2-54-234-242-35.compute-1.amazonaws.com
```

## Building SigmaOS

To build SigmaOS, run the following from the parent directory of the project
root directory:

```
$ cd sigmaos
$ ./build.sh
```

The build can be sped up by builiding binaries in parallel. WARNING: this will
try to consume as much CPU as your computer has, and may cause your machine to
slow down or OOM (so proceed with caution). Parallel builds can be run with:

```
$ ./build.sh --parallel
```

This build is suitable to run SigmaOS locally. In order to run deploy SigmaOS
on remote clusters (like AWS or CloudLab) and run remote benchmarks, you must
built with a `tag`. We have set up the `sosp24ae` tag for reviewers to use.
To build the remote version of SigmaOS, run:

```
$ ./build.sh --target aws --push sosp24ae
```

## Running experiments

Once SigmaOS has been built in remote mode, you can run its experiments. For
convenience, we provide a single script which runs the major experiments from
the paper. Note that this script expects a CloudLab cluster to already be set
up and running. To set up a CloudLab cluster, see the section that describes
how to do this below. From the root of the `sigmaos` repo, you can run the
experiments with:

```
$ ./artifact/sosp24/scripts/run-experiments.sh
```

We also provide a script which runs only the AWS-based experiments, and another
which runs only the CloudLab-based experiments, to fascilitate evaluating these
separately. Neither of these scripts expects that the other's cluster is up and
running. To run the AWS experiments, run:

```
$ ./artifact/sosp24/scripts/run-aws-experiments.sh
```

And for CloudLab experiments, run:

```
$ ./artifact/sosp24/scripts/run-cloudlab-experiments.sh
```

Each experiment's data is generated using one (or more) invocations of a Go
test program. If any of them fail, they can be rerun individually by deleting
the test's result output directory, and re-invoking the Go test program as
is done in the experiment runner script.

The scripts try to cache benchmark results to avoid rerunning them. In order
to force rerun a benchmark (particularly useful if the benchmark invocation
failed), remove the cached results. For example for the `start_latency`
benchmark, this can be done with:

```
$ rm -rf benchmarks/results/SOSP24AE/start_latency
```

Re-invoking the benchmark will then cause the results to be generated again,
from scratch.

Once all experiments have run successfully, the paper's corresponding graphs
can be generated with:

```
$ ./artifact/sosp24/scripts/generate-graphs.sh
```

By default, the resulting graph PDFs will be stored in the following directory,
relative to the root of the `sigmaos` repo:

```
benchmarks/results/graphs/
```

## Setting up a CloudLab cluster

The CloudLab cluster specs we used to evaluate SigmaOS can be found below.  For
convenience, we provide some scripts to set up a CloudLab cluster in the way
the SigmaOS scripts/benchmarks expect. Reviewers can run these from the
benchmark driver machine.

First, create your CloudLab cluster, and double-check that all machines can
communicate by running a linktest (we have had issues with this in the past).

Place a private key to access the cluster in the file
`sigmaos/cloudlab/cloudlab-sigmaos` (which is where our scripts will look for
it). Then, copy the AWS and Docker credentials provided in the artifact
evaluation package like so:

```
$ cp sigmaos-artifact-pkg/.aws/* sigmaos/aws/.aws
$ cp sigmaos-artifact-pkg/.docker/* sigmaos/aws/.docker
```

This should already be done on the benchmark driver machine.

Next, write the CloudLab Manifest description of your cluster to a file (in
this example, `/tmp/manifest.xml`). Then, run the following to ingest the
manifest in a format that our scripts expect:

```
$ cd cloudlab
$ ./import-server-manifest.sh --manifest /tmp/manifest.xml --username YOUR_CLOUDLAB_USERNAME
```

Then, change the `LOGIN` field in `cloudlab/env.sh` to your cloudlab username.

You can then set up your cloudlab cluster (and run any benchmarks) by running:

```
$ ./setup-cluster.sh
```

## Required Software/Hardware

Some of the paper experiments are run on AWS, and others are run on CloudLab.
Experiments performed on AWS require fast S3 access, whereas experiments
performed on CloudLab are tail-latency focused, and require kernel and hardware
configuration which, at the time of writing, is disallowed on AWS VMs. 
This section describes the Software and Hardware setup used in each setting.

### AWS

#### Summary of required resources

- 8 [m6i.4xlarge](https://aws.amazon.com/ec2/instance-types/m6i/) VMs
- 24 [t3.xlarge](https://aws.amazon.com/ec2/instance-types/t3/) VMs

#### Setup for each experiment

- Start-latency experiment
  - Number of VMs: 8
  - VM type: [m6i.4xlarge](https://aws.amazon.com/ec2/instance-types/m6i/)
    - 16vCPU, 2.9GHz Intel Ice Lake 8375C
    - 64GiB memory
    - 12.5Gbps network burst bandwitdh
    - 10Gbps EBS burst bandwidth
    - 20GB EBS storage
  - OS: Ubuntu Jammy 22.04

- Map Reduce experiment
  - Number of VMs: 8
  - VM type: [t3.xlarge](https://aws.amazon.com/ec2/instance-types/t3/)
    - 4vCPU (with 2vCPU disabled on each machine to make comparison to AWS
      Lambda fair)
    - 16GiB memory
    - 5Gbps network burst bandwidth
    - 2,085 Mbps EBS burst bandwidth
    - 20GB EBS storage
  - OS: Ubuntu Jammy 22.04

- Multi-realm image resize experiment
  - Number of VMs: 24
  - VM type: [t3.xlarge](https://aws.amazon.com/ec2/instance-types/t3/)
    - 4vCPU
    - 16GiB memory
    - 5Gbps network burst bandwidth
    - 2,085 Mbps EBS burst bandwidth
    - 20GB EBS storage
  - OS: Ubuntu Jammy 22.04

#### AWS Lambda experiments

### CloudLab

#### Summary of required resources

- 24 [c220g5](https://docs.cloudlab.us/hardware.html) nodes
  - CloudLab profile:
    [small-lan](https://www.cloudlab.us/p/PortalProfiles/small-lan)

#### Setup for each experiment

- Maximum spawn throughput experiment
  - Number of nodes: 24
  - Node configuration:
    - 40 logical cores
      - 2 Intel Xeon Silver 4114 10-core CPUs at 2.2GHz
    - 192GB ECC DDR4-2666 memory
    - Dual-port Intel X520DA2 10Gb NIC
    - 2,085 Mbps EBS burst bandwidth
    - 1TB 7200RPM 6G SAS HD
  - OS: Ubuntu Jammy 22.04

- Hotel and Socialnet application performance experiments
  - Number of nodes: 8
  - Node configuration:
    - 40 logical cores (8 physical cores enabled)
      - 2 Intel Xeon Silver 4114 10-core CPUs at 2.2GHz
    - 192GB ECC DDR4-2666 memory
    - Dual-port Intel X520DA2 10Gb NIC
    - 2,085 Mbps EBS burst bandwidth
    - 1TB 7200RPM 6G SAS HD
  - OS: Ubuntu Jammy 22.04

- Hotel and ImgResize multi-realm multiplexing experiment
  - Number of nodes: 8
  - Node configuration:
    - 40 logical cores (8 physical cores enabled)
      - 2 Intel Xeon Silver 4114 10-core CPUs at 2.2GHz
    - 192GB ECC DDR4-2666 memory
    - Dual-port Intel X520DA2 10Gb NIC
    - 2,085 Mbps EBS burst bandwidth
    - 1TB 7200RPM 6G SAS HD
  - OS: Ubuntu Jammy 22.04
