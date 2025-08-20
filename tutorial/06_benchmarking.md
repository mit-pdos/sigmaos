# 06. Benchmarking

This tutorial describes how to write and run SigmaOS benchmarks for remote
clusters of machines in the existing SigmaOS benchmarking framework. By the end
of this tutorial, you should be able to add SigmaOS benchmarks and run them
yourself.

Before continuing, make sure to go through the remote development
[tutorial](./02_remote_dev.md).

## `benchmarks/becnhmarks_test.go`: The master benchmark runner script

All remote SigmaOS benchmarks share a common entry point:
`benchmarks/benchmarks_test.go`. This test file contains stubs which, when
invoked try to connect to a SigmaOS cluster and run applications in it. It has
a LOT of benchmark-/application-specific configuration options and has grown in
complexity over time (apologies for the code quality), but this section
attempts to roughly describe some of its important components and patterns
which you can use to add new benchmarks to SigmaOS.

### Test stubs

An example test stub, `TestExample`, is available in
`benchmarks/benchmarks_test.go`. This stub will ultimately be run on the remote
cluster's client machine, and will connect to a running SigmaOS cluster, spawn
procs/start the SigmaOS applications to be benchmarked, run the benchmarks,
and save the results. For examples of multi-client benchmarks (e.g., benchmarks
in which a single client machine is insufficient to supply enough load to the
aplication), see `TestHotelSigmaosSearch` and `TestHotelSigmaosJustCliSearch`.
In these test cases, one client machine serves as the benchmark driver, and
other machines serve as followers.

## `benchmarks/remote`: The remote benchmark orchestration package

This package calls the appropriate `cloudlab` and `aws` scripts to fully set
up and tear down a SigmaOS cluster, automates invocation of the master
benchmark runner script, and collects any results it produces. This section
describes how to add a test stub to the package so that you can implement and
add benchmarks which leverage the existing benchmarking infrastructure.

### `benchmarks/remote/remote_test.go`: Entry point into the remote benchmark orchestration package.

The entry point into the remote benchmark orchestration package is
`benchmarks/remote/remote_test.go`. Examples of how it is invoked can be found
[here](../artifact/sosp24/scripts/run-cloudlab-experiments.sh).

Each benchmark has its own test stub (see `TestExample` for a simplified
example) which calls into the testing infrastructure to set up/tear down remote
SigmaOS clusters and invoke the master benchmark runner script on your behalf.
It passes a func to the remote benchmarking infrastructure, which contains a
string bash command to be executed on the remote benchmark client machine. This
usually is used to invoke the desired test stub from the master benchmark
runner script.

The remote benchmarking infrastructure allows you to also specify the size of
the cluster, its composition (e.g., number of `besched`-only nodes,
enabling/disabling the cluster CPU's TurboBoost), and the client machine on
which you wish to run the master benchmark runner script.

***TODO: write about output***

### `benchmarks/remote/benchcmds.go`: Benchmark command constructors

This file contains constructors for the bash commands which invoke the
benchmark runner script. You can control the benchmarks' output and performance
tracing by setting the perf/debug selectors in this your benchmark command
constructor, pass through flags, etc. See `GetExampleCmdConstructor` for an
example.
