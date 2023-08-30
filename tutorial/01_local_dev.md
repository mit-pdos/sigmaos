# 01. Local develpment

This tutorial describes how to start up SigmaOS locally. By the end of this
walkthrough, you should be able to build, run, and test SigmaOS. All commands
are intended to be run from the root of the repo.

## Dependencies

You will need to have `docker`, `mysql`, and `libseccomp-dev` installed in
order to build and run SigmaOS and its benchmarks. On a Ubuntu system, these
can be installed by running:

```
$ sudo apt install docker.io libseccomp-dev mysql-client
```

## Building SigmaOS Locally

We have two build configurations for SigmaOS: `local` and `aws`. `local` is
intended for development and correctness testing, whereas `aws` is intended for
performance benchmarking and multi-machine deployments (including CloudLab).
The primary differences are:
  - `local` builds the user `procs` directly into the `sigmaos` container,
    which enables offline development. `aws` builds the user `procs` in a
    dedicated build container and uploads them to an S3 bucket, omitting them
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

Warning: if the machine you are building on is small, this may cause your
machine to run out of memory, as the `go` compiler is somewhat
memory-intensive.

In order to make sure the installation succeeded, run a simple test which
starts SigmaOS up and exits immediately:

```
go test -v sigmaos/fslib --run InitFs --start
```

The output should look something like:

```
=== RUN   TestInitFs
13:44:32.833121 boot [sigma-7cfbce5e]
--- PASS: TestInitFs (3.42s)
```

## Testing SigmaOS

SigmaOS leverages Golang's testing infrastructure for its benchmarks
and correctness tests. We have an extensive slew of tests for many of
the SigmaOS packages. We expect all of the tests to pass, but we are
sure there must be bugs. If you find one, please add a minimal test
that exposes it to the appropriate package before fixing it. This way,
we can ensure that the software doesn't regress to incorporate old
bugs as we continue to develop it.

Occasionally, we run the full-slew of SigmaOS tests. In order to do so, run:

```
$ ./test.sh 2>&1 | tee /tmp/out
```

This will run the full array of tests, and save the output in `/tmp/out`.
However, running the full set of tests takes a long time.   For a
quick check run (which runs a few key tests for the main apps):

```
$ ./test.sh --apps --fast 2>&1 | tee /tmp/out
```


Generally, we only run a few tests related to packages we are actively
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

The `--boot` tells `mount.sh` to start SigmaOS; without the flag you
can mount an already-running SigmaOS.

This mounts the realm file system at `/mnt/9p`. On your computer
type `$ ls /mnt/9p/` and you should see output like this:

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

## Exercise: Access S3

Through the proxy you can access other SigmaOS services, such as AWS
S3.  For this exercise you need an AWS credential file in your home
directory `~/.aws/credentials`, which has the secret access key for
AWS, which we will post on Piazza.  Please don't share the key with
others and don't use it for personal use.

Once you have the key, do the following:
- [ ] Install credentials.  The entry in `~/aws/credentials`,
looks like this:
```
[me-mit]
aws_access_key_id = KEYID
aws_secret_access_key = SECRETKEY
region=us-east-1
```
- [ ] Stop SigmaOS
- [ ] Start SigmaOS

Now you should be able to access files in S3 by running:

```
ls /mnt/9p/s3/IP:PORT/
```

where IP:PORT is the IP address and port from `ls /mnt/9p/s3`.



