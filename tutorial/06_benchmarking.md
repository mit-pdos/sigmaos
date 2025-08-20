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
complexity over time (apologies for the code quality), but this suection
attempts to roughly describe some of its important components and patterns
which you can use to add new benchmarks to SigmaOS.
