# 01. Getting started

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

SigmaOS leverages Golang's testing infrastructure for its benchmarks and
correctness tests. We have an extensive slew of tests for many of the SigmaOS
packages. Although we are not aware of any major bugs, and expect all of the
tests to pass, we are sure there must be bugs. If you find one, please add a
minimal test that exposes it to the appropriate package before fixing it. This
way, we can ensure that the software doesn't regress to incorporate old bugs as
we continue to develop it.

Occasionally, we run the full-slew of SigmaOS tests. In order to do so, run:

```
$ ./test.sh -v 2>&1 | tee /tmp/out
```

This will run the full array of tests, and save the output in `/tmp/out`.
However, running the full set of tests takes a long time. Generally, we only
run a few tests related to packages we are actively developing. In order to run
an individual package's tests, begin by stopping any existing SigmaOS instances
and clearing the `go` test cache with:

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

The --start flag indicates to the test program that an instance of SigmaOS is
not already running. When benchmarking and testing remotely, you will likely
omit the `--start` flag. [Lesson 2](./02_remote_dev.md) explains the remote development
and benchmarking workflow in detail.

## Debugging SigmaOS

The SigmaOS [debug](../debug) package contains the SigmaOS logging
infrastructure. When running SigmaOS, you can control the logging output by
setting the `SIGMADEBUG` environment variable in the terminal you run SigmaOS
or its tests and benchmarks in. For example, in order to get output from the
test and benchmark packages, set:

```
$ export SIGMADEBUG="TEST;BENCH;"
```

This will make output from any logging statements with the `TEST` and `BENCH`
selectors print to stdout. For example, the following logging statements
will produce output, when `SIGMADEBUG` is set as above:

```
db.DPrintf(db.TEST, "Hello world 1");
db.DPrintf(db.BENCH, "Hello world 2");
```

The following logging statements, however, will _not_ produce output:

```
db.DPrintf(db.HOHO, "Hello world 3");
db.DPrintf(db.HAHA, "Hello world 4");
```

Most SigmaOS packages and layers contain their own logging levels. For a full
list, refer to the debbug package's [list of selectors](../debug/selector.go).

Test programs will direct logging output directly to your terminal. However,
SigmaOS kernel components and user `procs` run in containers. These store their
output elsewhere. In order to scrape all containers' logging output, run:

```
$ ./logs.sh
```

## Performance debugging

We have developed a variety of performance measurement tools for SigmaOS, built
on Golang's performance monitoring infrastructure. The performance measurement
tools are defined and implemented in the [perf](../perf/util.go) package.
Currently, the `perf.Perf` struct can be used to collect CPU, memory, mutex, or
blocking profiles from the `go` runtime. The resulting traces are compatible
with the go pprof tool. The Golang documentation has good writeups and docs
which describe how to read and interpret these. The following docs are
particularly useful:

  - [Profiling Go Programs](https://go.dev/blog/pprof): overview of Golang
    performance profiling.
  - [runtime/pprof](https://pkg.go.dev/runtime/pprof): documentation `pprof`,
    Golang's performance profiling infrastructure, on which much of our `perf`
    package was built.
  - [net/pprof](https://pkg.go.dev/net/http/pprof): documentation of the
    `net/pprof` package, which includes instructions to collect and view
    performance profiles over HTTP.

Similarly to the `debug` package, output from the `perf` package is controlled
through an environment variable `SIGMAPERF`. The full list of `perf` selectors
is available [here](../perf/selector.go). For example, in order to collect
a CPU pprof trace and a mutex trace for `named`, set:

```
$ export SIGMAPERF="NAMED_PPROF;NAMED_PPROF_MUTEX;"
```

In order to profile a SigmaOS `proc` or test program with selector `SELECTOR`,
create a new `perf.Perf` object like so:

```
p := perf.Perf(perf.SELECTOR)
```

In order to save the performance output, simply call (usually in a `defer`
statement):

```
p.Done()
```

The performance output will be available in
`/tmp/sigmaos-perf/PID-selector.out`, where `PID` is the SigmaOS `PID` of the
process, and `selector`is the lowercase version of the `perf` selector.

## Stopping SigmaOS

In order to stop SigmaOS and clean up any running containers, run:

```
$ ./stop.sh --parallel
```

Note: this will try to purge your machine of any traces of the running
containers, including logs. We do this to avoid filling your disk up, but you
may want to refrain from running `stop.sh` if you want to inspect the
containers' logs.
