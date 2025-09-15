# 01. Local develpment

This tutorial describes how to run SigmaOS locally. By the end of this
walkthrough, you should be able to build, run, and test SigmaOS. All
commands are intended to be run from the root of the repo.

## Dependencies

You will need to have `docker`, `docker buildx`, `mysql`, `parallel`,
`libseccomp-dev`, and `golang` v1.21 installed in order to build
and run SigmaOS and its benchmarks. 

In order to download Docker Desktop (which includes buildx, a plugin required
by the SigmaOS build sequence), follow the following guide:

```
https://docs.docker.com/desktop/install/ubuntu/
```

Follow the instructions from the following guide to install Go 1.21:

```
https://go.dev/doc/install
```

On a Ubuntu system, you can install the remaining packages by running:

```
$ sudo apt install libseccomp-dev mysql-client parallel
```

On a Ubuntu system, you may also have to install the SigmaOS AppArmor
profile, if you have AppArmor enabled. You can do this by running:

```
$ cd sigmaos
$ ./load-apparmor.sh
```

Note: `/var/run/docker.sock` must be accessible to SigmaOS. You can add your
user account to the docker group by running:

```
$ sudo usermod -aG docker $USER
```

In order for this to take effect, you will have to log out and back into your
machine. If you want to skip this, you can temporarily change the permissions
on the docker socket using:

```
$ sudo chmod 0666 /var/run/docker.sock
```

## Building SigmaOS Locally

We have two build configurations for SigmaOS: `local` and `aws`. `local` is
intended for development and correctness testing, whereas `aws` is intended for
performance benchmarking and multi-machine deployments (including CloudLab).
The primary differences are:
  - `local` builds all `procs` locally and stores them in the local host file
    system (rather than copying them to the target docker images) to speed up
    build times. `aws` copies some of the proc binaries (e.g. `linux` and
    `kernel` binaries) to the target docker images, and uploads user `proc`
    binaries to an AWS S3 bucket. This keeps the `sigmaos` container small,
    decreases its cold-start time, and mirrors a more realistic deployment
    scenario, in which the datacenter provider would download tenant's binaries
    into a generic `sigmaos` container before running them.
  - `local` has shorter session timeouts, more frequent heartbeats, and more
    frequent `raft` leader elections, which makes tests which stress
    fault-tolerance run faster. `aws` has longer timeouts, which decreases the
    system overhead when benchmarking. The exact hyperparameter settings can be
    found [here](../sigmap/hyperparams.go).
  - `local` keeps the built container images local, while `aws` pushes the
    container images to [DockerHub](https://hub.docker.com/) repos.

In order to build SigmaOS for local development, run the following command:

```
$ ./build.sh
```

*** NOTE: YOU SHOULD NOT NEED TO USE SUDO TO RUN `build.sh` . IF YOU ARE
TEMPTED TO... DON'T (ask for help debugging your config) ***

If you wish to speed up your build by building the binaries in parallel, run:

```
$ ./build.sh --parallel
```

Warning: the parallel build uses much memory and all but one core on the
machine you are building on.

There are many user procs in the repo, and building them all can take a long
time. If you wish to build only a subset of the user procs (e.g. `sleeper`
and `spinner`), you can specify which user procs to build in a comma-separated
list like so:

```
$ ./build.sh --userbin sleeper,spinner
```

In order to speed up builds, SigmaOS runs a Golang builder container and a Rust
buidler container in the background, with the root directory of the repo
mounted. These builder containers should only need to be restarted and rebuilt
if their Dockerfile definitions change (which is expected to happen very
rarely, if ever). To rebuild and restart the builder containers, run:

```
$ ./build.sh --rebuildbuilder
```

## SigmaOS run dependencies

SigmaOS uses `etcd` for fault-tolerant storage and you may have to (re)start
etcd:

```
$ ./start-etcd.sh
```

You can check if `etcd` is running and accessible as follows:
```
$ docker exec etcd-server etcdctl version
```

Also, sigmaos expects the directories `~/.aws` and `/mnt/9p` to exist when it
runs. To create them, run:

```
$ mkdir ~/.aws
$ mkdir /mnt/9p
$ sudo chown $USER /mnt/9p
```

SigmaOS also expects to find valid AWS `credentials` and `config` set up in
your user account's AWS config directory, `~/.aws`. The `credentials` and
`config` files contain the AWS configuration and secret access keys required
for SigmaOS to access your AWS resources. The AWS CLI docs contain information
on how to set up your `config` and `credentials` files:

```
https://docs.aws.amazon.com/cli/v1/userguide/cli-configure-files.html
https://docs.aws.amazon.com/cli/v1/userguide/cli-chap-configure.html
```

SigmaOS expects there to be a `sigmaos` entry in your AWS `credentials` and
`config` files. The entry in `~/aws/credentials` looks like this:

```
[sigmaos]
aws_access_key_id = KEYID
aws_secret_access_key = SECRETKEY
region=us-east-1
```

If you have an AWS account, you can replace `KEYID` and `SECRETKEY`
with your account's key.  If you don't have an account, you will need to create
one (google create an AWS account).

You can check whether you set up your AWS credentials correctly by installing
the AWS CLI (`sudo apt install awscli`) and running:

```
$ aws s3 ls --profile sigmaos
```

In order to make sure the build succeeded, run a simple test which starts
SigmaOS up and exits immediately:

```
$ go test -v sigmaos/sigmaclnt/fslib --run InitFs --start
```

The output should look something like:

```
=== RUN   TestInitFs
13:44:32.833121 boot [sigma-7cfbce5e]
--- PASS: TestInitFs (3.42s)
```

## Testing SigmaOS

SigmaOS leverages Golang's testing infrastructure for its benchmarks and
correctness tests. We have tests for many of the SigmaOS packages. We expect
all of the tests to pas, though some tests require access to our private S3
buckets and will only pass once we have given you access to them. We have
not tested extensively on different hardware setups or OS versions, and we are
sure there must be bugs. If you find a bug, please add a minimal test that
exposes it to the appropriate package before fixing it. This way, we can ensure
that the software doesn't regress to incorporate old bugs as we continue to
develop it.

Occasionally, we run the full-slew of SigmaOS tests. In order to do so, run:

```
$ ./test.sh 2>&1 | tee /tmp/out
```

This will run the full array of tests, and save the output in
`/tmp/out`.  However, running the full set of tests takes a long time.
To run a few key tests for the main apps, run:

```
$ ./test.sh --apps-fast 2>&1 | tee /tmp/out
```

We try to do a good job of cleaning up state between tests, even if the tests
fail. However, if a package's test(s) fail while running `./test.sh`, there is
a chance that some SigmaOS state will be left hanging around, causing
subsequent packages' tests to fail. In order to ensure that all SigmaOS state
is fully cleaned up between running different packages' tests, run:

```
$ ./test.sh --cleanup 2>&1 | tee /tmp/out
```

Generally, we run only tests related to packages we are actively
developing. In order to run an individual package's tests, begin by
stopping any existing SigmaOS instances and clearing the `go` test
cache with:

```
$ ./stop.sh --parallel
$ go clean -testcache
```

Then, start the package's tests by running:

```
$ go test -v sigmaos/<pkg_name> --start
```

In order to run a specific test from a package, run:

```
$ go test -v sigmaos/<pkg_name> --start --run <test_name>
```

The --start flag indicates to the test program that an instance of SigmaOS must
be started. When benchmarking and testing on a real cluster, you will likely
omit the `--start` flag. [Lesson 2](./02_remote_dev.md) explains the remote
development and benchmarking workflow in detail.

## Exercise: Start SigmaOS

In this exercise, you will start SigmaOS and introspect it.  SigmaOS leverage's
Linux's 9P VFS layer to allow interaction with SigmaOS via the command line. In
order to do so, we implemented a 9P-to-SigmaP proxy `proxyd`. First find your
machine's local IP by running:

```
$ hostname -I
```

Make sure the directory `/mnt/9p` exists and then run:

```
$ ./mount.sh --boot LOCAL_IP
```

This starts up a fresh SigmaOS instance, and mounts the root
realm's `named` at `/mnt/9p`. You should see output like this:
```
$ ./mount.sh --boot 127.0.0.1
..........................192.168.0.10 container 20a7be3eb7 dbIP x.x.x.x mongoIP x.x.x.x

```

The `--boot` tells `mount.sh` to start SigmaOS. In order to introspect an
already-running SigmaOS instance, run `mount.sh` without this flag. 

You can `ls` the root directory of `named` as follows:
```
$ ls /mnt/9p/
```
and you should see output like this:
```
$ ls /mnt/9p/
boot  db  kpids  named-election-rootrealm  rpc  s3  schedd  ux  ws
$ 
```

Make sure to stop the SigmaOS instance, as described below,
before re-running `./mount.sh` or running any tests. Running two
instances of SigmaOS on the same machine is likely to result in
unexpected errors and/or hangs.

## Stopping SigmaOS

In order to stop SigmaOS and clean up any running containers, run:

```
$ ./stop.sh --parallel
```

Note: this will try to purge your machine of any traces of the running
containers, including logs and cached build images. We do this to avoid filling
your disk up, but you may want to refrain from running `stop.sh` if you want to
inspect the containers' logs. Additionally, you may wish to preserve the Docker
build cache to speed up your builds (but still delete the logs). In order to do
so, you can run:

```
$ ./stop.sh --parallel --nopurge
```

In general, we run `./stop.sh` with the `--nopurge` flag before running any new
set of tests to ensure that SigmaOS starts from a clean state.

## Exercise: Access S3 through SigmaOS

Through SigmaOS you can access other services, such as AWS S3. Ff you boot and
mount SigmaOS, you should be able to access files in S3 by running:

```
ls /mnt/9p/s3/SERVER_ID/
```

where `SERVER_ID` is the SigmaOS-generated ID for the S3 server. You can find
this by running:

```
ls /mnt/9p/s3
```

You can copy files into s3. For example,
```
cp tutorial/01_local_dev.md /mnt/9p/s3/SERVER_ID/<YOUR_BUCKET_NAME>/x
```
copies this tutorial file into the s3 object `x`.

Having access to s3 is convenient for building applications; see
exercises in [API tutorial](03_sigmaos_api.md).  In addition to the s3
proxy, SigmaOS also provides proxies for databases and the local file
system in each SigmaOS container (through `ux`).
