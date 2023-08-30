# 03. Code walkthrough

The SigmaOS codebase is large. This tutorial is a short walkthrough of
the code from both the client and server perspective. By the end of
this tutorial you should understand the different layers of SigmaOS
clients and servers, and how the pacakges that implement them fit
together.  This tutorial is intended for developers who want to modify
SigmaOS itself.

XXX TODO: insert diagram.

The following libraries make up the "base" on which the SigmaOS client-side is
built:
  - `sessclnt`: This library implements SigmaOS's session layer which
    wraps TCP connections provided in a session. If the TCP connection
    fails, the session layer will retry re-establishing the
    connection, perhaps to another server for the service.
  - `netclnt`: This library abstracts TCP connections from the client
    perspective, and is used to send messages from the client to the server,
    and deliver responses.

## Server-side libraries

This section describes the SigmaOS server-side libraries. It is not an
exhaustive list, but it contains some of the more interesting design points,
and libraries which may be useful for future projects based on SigmaOS.
  - `netsrv`: This layer abstracts TCP connections from the server perspective,
    and is used to receive messages from the client and send responses back.
  - `sesssrv`: This library contains the server-side session protocol, which
    collaborates with `sessclnt` to decide when a session is done.
  - `sessstatesrv`: This library contains server-side data structures needed to
    represent, track, and manage the lifecycle of sessions.
  - `protsrv`: This library implements a generic SigmaOS protocol server. It
    has handlers for each of teh `sigmap` messages, and deals with SigmaOS
    features like versions, ephemeral files, and watches.

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

## Exercises

The following exercises will help you get familiar with the core of the SigmaOS
codebase.

### Exercise 1: Describe SigmaOS librarly layers

In this exercise, you will draw a diagram of how the SigmaOS client and server
libraries stack together, and explain what their purposes are. You should hit
the following waypoints in your traversal:
  - [ ] Start at a simple `fslib` function like `FsLib.Create`.
  - [ ] You should eventually reach the `sessclnt` layer, and then the
    `netclnt` layer. This is the bottom of the client stack, and marks the
    transition point between client and server code.
  - [ ] Start at the `netsrv`layer, and proceed into the `sesssrv` layer.
  - [ ] You should end up in the `protsrv` layer which corresponds to the
    `FsLib` function you started at.
  - [ ] Now, draw a diagram of how all of the libraries stack and fit together.
  - [ ] Finally, describe, at a high level, what each library does.

### Exercise 2: Add a protocol message to SigmaP 

In this exercise, you will add a new type of `sigmap` message to SigmaOS. It
can be a no-op, or print something on the server-side. In order to do so,
you'll need to complete the following major steps (with some details left out):
  - [ ] Create a new type of `sigmap` message, and add it to the `sigmap`
    package.
  - [ ] Add an API call for the new RPC to the `fslib.FsLib` struct, and the
    lower layers it calls into, in order to invoke your RPC. You should be able
    to trace your way all the way down from the `fslib` layer to the `netclnt`
    layer.
  - [ ] Add a handler for your new `sigmap` message to `protsrv`. You should be
    able to trace your RPC's flow all the way from the `netsrv` layer to the
    `protsrv` layer.
  - [ ] Invoke your new `sigmap` message on `named` in a test, and ensure that
    it works.

