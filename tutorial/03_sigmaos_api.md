# 03. SigmaOS APIs

This tutorial helps you writing applications with SigmaOS by making you
familiar with its main APIs.  Applications (user procs) using SigmaOS live in
`cmd/user`.  The root directory contains the support packages for the major
applications: `mr` (a MapReduce Library), `hotel` and `socialnetwork` (two
microservices based on DeathStarBench), `imgresized` (an image resizing
service), and `kv` (a sharded key-value service).  The exercises below will
help you get familiar with the SigmaOS APIs; the last one, exercise 4, puts the
earlier exercises together into a simple application.

## Client-side libraries

This section describes the SigmaOS client-side interface and the libraries that
implement it. The following list of libraries is not exhaustive (many of them
call into other libraries), but most SigmaOS clients and `procs` will use only
a subset of these libraries:
  - `fslib`: This is the main library that all clients use to interact in
    SigmaOS. It defines common file-system operations (like `Open`, `Write`,
    `Read`, `Close`).
  - `procclnt`: This library designs and implements the `proc` API. `proc`s,
    benchmarks, and tests use the `procclnt` API to `Spawn`, `Evict`, and
    `Wait` for `proc`s.
  - `sigmaclnt`: This library unifies the `fslib` and `procclnt` structures
    into a single interface. It is mostly for convenience.
  - `semclnt`: This library defines the SigmaOS equivalent of semaphores.
  - `leaderclnt`: This library uses `electclnt` to implement leader
    election, and fence directories and services with the leader's
    epoch.
  - `rpcclnt`: This library implements general-purpose RPCs on top of
    SigmaOS.

## Server-side libraries

This section describes the SigmaOS server-side libraries. It is not an
exhaustive list, but it contains some of the more interesting design
points, and libraries, which may be useful for future projects based
on SigmaOS.
  - `sigmasrv`: This library provides the API to create SigmaOS servers
  - `protsrv`: This library implements a generic SigmaOS protocol server. It
    has handlers for each of teh `sigmap` messages, and deals with SigmaOS
    features like watches.

## Shared libraries (used by client and server)

This section describes some libraries shared by the SigmaOS
client-side libraries and the SigmaOS server-side libraries, which may
be useful when implementing additional clients and servers:
  - `sigmap`: This library defines the `sigmap` protocol, which all components
    and `procs` in SigmaOS use to communicate. It is loosely based on the 9P
    protocol, with some additions for fault-tolerance (such as `Watch`es) and
    performance (such as `Put`, `Get`, and `WriteRead`).
  - `sessp`: This library defines messages for the session layer of SigmaOS.

### Exercise 1: Create, write, and read files in named

In this exercise, you will learn how to use the SigmaOS client API to
manipulate files and directories. In order to do so, you will create a file,
write data to it, and read data from it. You will need to complete the
following steps:
  - [ ] Create a file `tfile` in `named`. The pathname `name/` names
    the root directory of `named` and thus use the pathname
    `name/tfile` for `Create`. Write a string of your choice to it,
    and close the file.
  - [ ] List the contents of the directory in which you created the file.
    Ensure the file is present.
  - [ ] Open the file, and read the contents back. Make sure that the contents
    you read match the contents you wrote.
    
To get started, open `example_test.go` in the folder `example`, which
contains the `TestExerciseNamed` function, a Golang test function, to
read the root directory of `named`.  The call to `test.MakeTstatePath`
starts a test instance of SigmaOS with only named. The `ts` instance
embeds an `SigmaClnt` object, which in turn embeds an `FsLib` object.
You can run the test as follows:

```
$ go test -v sigmaos/example --start --run Named
```

and it will produce output like this:
```
=== RUN   TestExerciseNamed
10:50:56.668788 name/: [.statsd rpc boot chunkd db kpids lcsched memfs mongo procq realm s3 schedd ux]
--- PASS: TestExerciseNamed (0.61s)
```

Note: when running tests, it is recommendable to first run `./stop.sh` first,
to make sure any SigmaOS containers/instances which left haning around are
cleaned up before the test starts. Otherwise, your tests may hang. One
short-hand for doing this (which we commonly use during development) is the
following:

```
$ ./stop.sh; go test -v sigmaos/example --start --run Named
```

Now extend `TestExerciseNamed` to implement the exercise.
`fslib/fslib_test.go` and `fslib/file.go` may provide inspiration.
(If you are unfamiliar with Golang, check the out [Go
tutorial](https://go.dev/doc/tutorial/getting-started).

Note that the state stored in the `named` root directory is persistent; `named`
uses an `etcd` for storage, which is a widely-used, fault-tolerant, key-value
server implemented using Raft.  So, your test should clean up after itself,
because, otherwise, if you run it again, it will fail, because your file
already exists. To manually clear out `etcd`, run:

```
$ ./fsetcd-wipe.sh
```

### Exercise 2: Read a file from S3
    
SigmaOS's `named` is good for storing small files (e.g., symbolic links that
servers use to advertise their existence). SigmaOS has proxy servers to access
other storage systems, including AWS S3.  Each machine in SigmaOS runs an `s3`
proxy and you can read/write files in S3 using the pathname `name/s3/~any/`
(`any` tells SigmaOS to use any of the available S3 proxies in `name/s3`).

For this exercise you need to have your AWS credential files set up, according
to [local](01_local_dev.md). You may also have to copy the files in `input`
into an S3 bucket of your creation, at the path `<YOUR_BUCKET_NAME>/gutenberg`.

Using the same FsLib interface as in the previous exercise, extend
`TestExerciseS3` to:
  - [ ] Read the file
  `name/s3/~any/<YOUR_BUCKET_NAME>/gutenberg/pg-tom_sawyer.txt`
  - [ ] Count the number of occurrences of the word `the` in this file
    
Note that `test.MakeTstateAll` creates an instance of SigmaOS with `named` and
other kernel services (such as `s3` proxies).

Hint: The function `OpenReader` from `FsLib` along with Golang's `NewScanner`
and `scanner.Split(bufio.ScanWords)` may be helpful.

### Exercise 3: Spawn a `proc`

In this exercise, you will familiarize yourself with the `procclnt` API.  The
function `TestExerciseProc` spawns the example proc from `cmd/user/example/`.
The test function runs this proc using `Spawn`, which queues the proc for
execution. The test function waits until the proc starts, and then waits until
it exits. 

The proc in `cmd/user/example/` makes an `SigmaClnt` object, tells SigmaOS it
started using `Started`, prints "Hello World", and then exits using
`ClntExitOK`.

If you run ```go test -v sigmaos/example --start --run Proc```, you should see
output like this:

```
=== RUN   TestExerciseProc
    example_test.go:62: 
                Error Trace:    /home/kaashoek/hack/sigmaos/example/example_test.go:62
                Error:          Not equal: 
                                expected: "Hello world"
                                actual  : ""
                            
                                Diff:
                                --- Expected
                                +++ Actual
                                @@ -1 +1 @@
                                -Hello world
                                +
                Test:           TestExerciseProc
--- FAIL: TestExerciseProc (6.33s)
```

Test programs will direct logging output directly to your terminal. However,
SigmaOS kernel components and user `procs` run in containers. These store their
output elsewhere. In order to scrape all containers' logging output, run:

If you run
```
$ ./logs.sh
```
search for "Hello world" in the output and you will the print
statement from the example proc.

Modify the example proc to return `hello world` its exit status:
  - [ ] Edit the `main` function in `cmd/user/example/main.go` and replace
    `ClntExitOK` with `ClntExit`, passing in the appropriate `proc.Status`
    using `MakeStatusInfo`.
  - [ ] Recompile and build SigmaOS: ```$./build.sh --parallel```. It is
    sometimes convenient to just the compile the SigmaOS programs on the local
    machine before running the full `build.sh` sequence, to see if
    there are any compilation errors. This can be done by running:
    ```
    $./make.sh --norace user
    ``` 
    Or compile an individual user program:
    ```
        go build -ldflags="-X sigmaos/sigmap.Target=local" -o bin/user/example cmd/user/example/main.go
    ```
    Once they compile correctly, run build.sh.
    Note that the full `build.sh` sequence must be run before
    rerunning the test, for local changes to be reflected in the SigmaOS
    container images.
  - [ ] Rerun the test to see if your implementation now passes the test.

### Exercise 4: Process data in parallel

This exercise puts the previous exercises together into a simple application
with several procs. Your job is to implement `TestExerciseParallel` to process
the input files in `name/s3/~any/<YOUR_BUCKET_NAME>/gutenberg/` in parallel:
  - [ ] Modify the example proc in `cmd/user/example` to take as argument a
    pathname for an input file (using Golang's `os.Args`), counts the
    occurrences of the world `the` in that file, and returns the number of
    words through `proc.Status`.  Your code from Exercise 2 may be helpful.
  - [ ] Modify the `TestExerciseParallel` to spawn an example proc for each
    input file, wait until they exited, and add up the number of `the`'s.  To
    make the procs run in parallel, you may want to create a go routine for
    each spawn, using Golang's `go`, and collect the results from go routine
    using a channel.

If you would run this test in the remote-mode configuration
[remote](02_remote_dev.md) of SigmaOS, SigmaOS would schedule the
procs on different machines for you.
    
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
selectors print to stdout. For example, the following logging statements will
produce output, when `SIGMADEBUG` is set as above:

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

### Exercise 5: Run and extend an RPC server. 

In this exercise, you will familiarize with SigmaOS RPC, specifically `rpcclnt`
and `sigmasrv` by running a simple RPC server that echo its input:
  - [ ] Navigate to the `example_echo_server` directory. Check the files and
    try running the test cases. If you have already built SigmaOS through
    `build.sh`, you may run `go test sigmaos/example_echo_server -v --start`.
    Overall, the test case starts an instance of the Echo server, then starts a
    client sending request to the server. By default, all operations are local. 
  - [ ] To see the logs, source the environment variable file
    `example_echo_server/echo_env.sh` before running test, and run `logs.sh`
    afterwards. You may modify the content of the environment variable file to
    turn on/off logging for different modules.  After finishing test and
    logging, you may run `stop.sh` to clear up.

Extend the client and server to support addition: the client sends two numbers
to the server and server responds with the sum:
 - [ ] Add the handler to echosrv.go and run `build.sh` to build server
 - [ ] Write a new test function that tests the new RPC

You can use `echo_env.sh` to set SIGMADEBUG.

### Optional exercises for RPC server
  - [ ] Try to modify the echo server so that it caches results by connecting
    to some caching client. Existing caching implementations can be found at
    `cacheclnt`, `memcached`, and `kv`. Example usage can be found at `hotel`
    and `socialnetwork`, which are two major example applications built on top
    of SigmaOS.
  - [ ] Try to modify the echo server so that it reads and writes to a database
    by connecting to a database proxy. Existing implementations can be found at
    `dbd` and `dbclnt`.  
  - [ ] Try to profile the echo server through `perf` package, described below.

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

