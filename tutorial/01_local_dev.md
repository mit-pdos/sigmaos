# 01. Local develpment

This tutorial describes how to run SigmaOS locally. By the end of this
walkthrough, you should be able to build, run, and test SigmaOS. All
commands are intended to be run from the root of the repo.

## Dependencies

You will need to have `golang`, `docker`, `mysql`, and `libseccomp-dev` installed in
order to build and run SigmaOS and its benchmarks. On a Ubuntu system, these
can be installed by running:

```
$ sudo apt install golang-go docker.io libseccomp-dev mysql-client
```

Note: `/var/run/docker.sock` must be accessible to SigmaOS, so you may
have to run:
```
sudo chmod 666 /var/run/docker.sock
```

## Building SigmaOS Locally

We have two build configurations for SigmaOS: `local` and
`aws`. `local` is intended for development and correctness testing,
whereas `aws` is intended for performance benchmarking and
multi-machine deployments (including CloudLab).  The primary
differences are:
  - `local` builds the user `procs` directly into the `sigmaos` container,
    which enables offline development. `aws` builds the user `procs` in a
    dedicated build container and uploads them to an AWS S3 bucket, omitting them
    from the `sigmaos` container. This keeps the `sigmaos` container small,
    decreases its cold-start time, and mirrors a more realistic deployment
    scenario, in which the datacenter provider would download tenant's binaries
    into a generic `sigmaos` container before running them.
  - `local` has shorter session timeouts, more frequent heartbeats, and more
    frequent `raft` leader elections, which makes tests which stress
    fault-tolerance run faster. `aws` has longer timeouts, which decreases the
    system overhead when benchmarking. The exact hyperparameter settings can be
    found [here](../sigmaps/hyperparams.go).
  - `local` keeps the built container images local, while `aws` pushes the
    container images to [DockerHub](https://hub.docker.com/) repos.

In order to build SigmaOS for local development, run the following command:

```
$ ./build.sh
```

If you wish to speed up your build by building the binaries in parallel, run:

```
$ ./build.sh --parallel
```

Warning: the parallel build uses much memory and all the cores on the
machine you are building on.

In order to make sure the build succeeded, run a simple test which
starts SigmaOS up and exits immediately:

```
go test -v sigmaos/fslib --run InitFs --start
```

Make sure that a directory `.aws` exists in your home directory.

The output should look something like:

```
=== RUN   TestInitFs
13:44:32.833121 boot [sigma-7cfbce5e]
--- PASS: TestInitFs (3.42s)
```

SigmaOS uses `etcd` for fault-tolerant storage and you may have to (re)start etcd:
```
./start-etcd.sh
```

You can check if `etcd` is running as follows:
```
docker exec etcd-server etcdctl version
```

## Testing SigmaOS

SigmaOS leverages Golang's testing infrastructure for its benchmarks
and correctness tests. We have tests for many of the SigmaOS
packages. We expect all of the tests to pass, but we are sure there
must be bugs. If you find one, please add a minimal test that exposes
it to the appropriate package before fixing it. This way, we can
ensure that the software doesn't regress to incorporate old bugs as we
continue to develop it.

Occasionally, we run the full-slew of SigmaOS tests. In order to do so, run:

```
$ ./test.sh 2>&1 | tee /tmp/out
```

This will run the full array of tests, and save the output in
`/tmp/out`.  However, running the full set of tests takes a long time.
To run a few key tests for the main apps, run:

```
$ ./test.sh --apps --fast 2>&1 | tee /tmp/out
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

The --start flag indicates to the test program that an instance of
SigmaOS must be started. When benchmarking and testing on a real cluster, you will likely
omit the `--start` flag. [Lesson 2](./02_remote_dev.md) explains the remote development
and benchmarking workflow in detail.

## Exercise: Start SigmaOS

In this exercise, you will start SigmaOS and introspect it.  SigmaOS
leverage's Linux's 9P VFS layer to allow interaction with SigmaOS via
the command line. In order to do so, we implemented a 9P-to-SigmaP
proxy `proxyd`. First find your machine's local IP by running:

```
$ hostname -I
```

Create the directory `/mnt/9p` and then, run:

```
$ ./mount.sh --boot LOCAL_IP
```

This mount the root realm's `named` at `/mnt/9p`. 
You should see output like this:
```
$ ./mount.sh --boot 127.0.0.1
..........................192.168.0.10 container 20a7be3eb7 dbIP x.x.x.x mongoIP x.x.x.x
08:03:08.702140 - ALWAYS Etcd addr 127.0.0.1

```

The `--boot` tells `mount.sh` to start SigmaOS; without the flag you
can mount an already-running SigmaOS. 

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

## Stopping SigmaOS

In order to stop SigmaOS and clean up any running containers, run:

```
$ ./stop.sh --parallel
```

Note: this will try to purge your machine of any traces of the running
containers, including logs. We do this to avoid filling your disk up, but you
may want to refrain from running `stop.sh` if you want to inspect the
containers' logs.

## Exercise: Access S3 through SigmaOS

Through SigmaOS you can access other services, such as AWS S3.  For
this exercise you must have an AWS credential file in your home
directory `~/.aws/credentials`, which has the secret access key for
AWS.  The entry in `~/aws/credentials` looks like this:
```
[sigmaos]
aws_access_key_id = KEYID
aws_secret_access_key = SECRETKEY
region=us-east-1
```

If you have an AWS account, you can replace `KEYID` and `SECRETKEY`
with your account's key.  If you don't have an account, you can create
one (google create an AWS account) or use the account key provided by
us (which we will post on Piazza).

Now you should be able to access files in S3 by running:

```
ls /mnt/9p/s3/IP:PORT/
```
where IP:PORT is the IP address and port from `ls /mnt/9p/s3`.

You can copy files into s3. For example,
```
cp tutorial/01_local_dev.md /mnt/9p/s3/192.168.0.10\:46043/x
```
copies this tutorial file into the s3 object `x`.

Having access to s3 is convenient for building applications; see
exercises in [API tutorial](03_sigmaos_api.md).  In addition to the s3
proxy, SigmaOS also provides proxies for databases and the local file
system in each SigmaOS container (through `ux`).
