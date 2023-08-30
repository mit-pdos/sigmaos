# 03. SigmaOS APIs

This tutorial help you writing applications with SigmaOS by making you
familiar with the main APIs.  Applications using SigmaOS live in
`cmd/user`.  The packages for the major applications are `mr` (a
MapReduce Library), `hotel` and `socialnetwork` (two microservices
based on DeathStarBench), `imgresized` (an image resizing service),
and `kv` (a sharded key-value service).

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
exhaustive list, but it contains some of the more interesting design points,
and libraries which may be useful for future projects based on SigmaOS.
  - `sigmasrv`: This library provides the API to create SigmaOS servers
  - `protsrv`: This library implements a generic SigmaOS protocol server. It
    has handlers for each of teh `sigmap` messages, and deals with SigmaOS
    features like watches.

## Shared libraries (used by client and server)

This section describes some libraries shared by the SigmaOS client-side
libraries and the SigmaOS serer-side libraries, which may be useful when
implementing additional clients and servers:
  - `sigmap`: This library defines the `sigmap` protocol, which all components
    and `procs` in SigmaOS use to communicate. It is loosely based on the 9P
    protocol, with some additions for fault-tolerance (such as `Watch`es) and
    performance (such as `Put`, `Get`, and `WriteRead`).
  - `sessp`: This library defines messages for the session layer of SigmaOS.

## Exercises

The following exercises will help you get familiar with the core of the SigmaOS
codebase.

### Exercise 1: Create, write, and read files in named

In this exercise, you will learn how to use the SigmaOS client API to
manipulate files and directories. In order to do so, you will create a file,
write data to it, and read data from it. You will need to complete the
following steps:
  - [ ] Create an `fslib.FsLib` object.
  - [ ] Create a file in `named`. Write a string of your choice to it, and
    close the file.
  - [ ] List the contents of the directory in which you created the file.
    Ensure the file is present.
  - [ ] Open the file, and read the contents back. Make sure that the contents
    you read match the contents you wrote.
    
To get started, open `example_test.go` in the folder `example`, which
contains the `TestExerciseNamed` function, a Go test function, to read
the root directory of `named`.  The call to `test.MakeTstatePath`
starts a test instance of SigmaOS with only named, and it embeds an
`SigmaClnt` instance, which embeds an `FsLib` object.  You can run the
test as follows:

```
$ go test -v sigmaos/example --start --run Named
```

and it will produce output like this:
```
20:42:16.638701 - ALWAYS Etcd addr 127.0.0.1
=== RUN   TestExample
20:42:17.059591 - BOOT Start: sigma-c69e2af8 srvs named IP 192.168.0.10
20:42:17.116655 name/: [.statsd rpc boot db kpids s3 schedd ux]
--- PASS: TestExample (0.51s)
PASS
ok      sigmaos/example 0.515s
```

Now extend `TestExerciseNamed` to implement the exercise.
`fslib/fslib_test` has many `fslib` tests, which may provide
inspiration.

Note that the state stored in the `named` root directory is
persistent; `named` uses an `etcd` for storage, which is a
widely-used, fault-tolerant, key-value server implemented using Raft.
So, your test should clean up after itself, because, otherwise, if you
run it again, it will fail, because your file already exists.

### Exercise 2: Read a file from S3
    
Named is a for storing small files (e.g., symbolic links that servers
use to advertise their existence). SigmaOS has proxy server to access
other storage systems such as S3.  Each machine in SigmaOS runs an
`s3` server and you read/write files in S3 using the pathname
`name/s3/~any/`, which tells SigmaOS to use any of the available S3
proxies. 

In this exercise, you will read a file in S3 using the same FsLib
interface as in the previous exercise.  Extend `TestExerciseS3`
to:
  - [ ] Read the file `name/s3/~any/9ps3/gutenberg/pg-tom_sawyer.txt`
  - [ ] Count the number of occurrences of the word `the`
    
Note that `test.MakeTstateAll` creates an instance of SigmaOS with `named`
and other kernel services (such as `s3` servers).

For this exercise you need an AWS credential file in your home
directory `~/.aws/credentials`, which has the secret access key for
AWS, which we will post on Piazza.
    
### Exercise 3: Spawn a `proc`

In this exercise, you will familiarize yourself with the `procclnt`
API.  The function `TestExerciseProc` spawns the example proc from
`cmd/user/example`, which queues it for execution. The test function
wait until it starts (if many procs are spawned, SigmaOS may not start
the proc for a while), and then wait until exits.

If you run ```go test -v sigmaos/example --start --run Proc```, you
should see output like this:
```
=== RUN   TestExerciseProc
08:30:58.202494 - BOOT Start: sigma-b7e137ea srvs all IP 192.168.0.10
08:30:58.276848 test-test-5abbed8715d06e97 ALWAYS Appended named 127.0.0.1
    example_test.go:53: 
                Error Trace:    /home/kaashoek/hack/sigmaos/example/example_test.go:53
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
--- FAIL: TestExerciseProc (6.14s)
```

Test programs will direct logging output directly to your
terminal. However, SigmaOS kernel components and user `procs` run in
containers. These store their output elsewhere. In order to scrape all
containers' logging output, run:

If you run,
```
$ ./logs.sh
```
search for "Hello world" in the output and you will the print
statement from the example proc.

Modify the example program to return `hello world` its exit status and
run it:
  - [ ] Edit the `main` function in `cmd/user/example` and replace
        `ClntExitOK` with `ClntExit`, passing in the appropriate
        `proc.Status` using `MakeStatusInfo`.
  - [ ] Recompile and build SigmaOS:  ```$./build.sh --parallel```
    It is sometimes convenient to just the compile the SigmaOS programs to see
    if they compile:  ```$./make.sh --norace user```, which compiles
    the user programs.  Once they compile, run build.sh.
  - [ ] Rerun the test

### Exercise 4: Process data in parallel

Implement `TestExerciseParallel` to process the input files
in `name/s3/~any/9ps3/gutenberg/` in parallel:
  - [ ] Make a proc that takes as argument a pathname for an input
    file, counts the occurrences of the world `the`, and returns it
    through `proc.Status`.  Make a new directory in `cmd/user` for the
    proc. Your code from Exercise 2 may be helpful.
  - [ ] Modify the test function to spawn a proc for each input file,
    wait until they exited, and add up the number of `the`'s.
    You can create a Go routine for each spawn.
  
### Exercise 5: Set up a RPC server. 

In this exercise, you will familiarize with SigmaOS RPC, specifically
`rpcclnt` and `sigmasrv`. In order to do so, you will learn how to set
up a basic RPC server, and explore existing utilities that provide
database and cache proxies.
  - [ ] Navigate to the `example_echo_server` directory. Check the files and 
	try running the test cases. If you have already built SigmaOS through `build.sh`, 
	you may run `go test sigmaos/example_echo_server -v --start`. Overall, the test
	case starts an instance of the Echo server, then starts a client sending request
	to the server. By default, all operations are local. 
  - [ ] To see the logs, source the environment variable file `example_echo_server/env.sh`
	before running test, and run `logs.sh` afterwards. You may modify the content
	of the environment variable file to turn on/off logging for different modules. 
	After finishing test and logging, you may run `stop.sh` to clear up.
  - [ ] Try to figure out how custome RPCs work. You may start with the `RPC` method
	in `rpcclnt.RPCClnt` and will eventually end up in `protclnt` and
	`netclnt` as in the previous exercises. On the server side, check how 
	`sigmasrv.SigmaSrv` is implemented and eventually you will reach `sesssrv`
	and `netsrv` 
  - [ ] Try to figure out how the test case initializes the SigmaOS kernel and 
	a proc for the Echo server. You may check utilities in the `test` directory, 
	and also a `main` function defined at `cmd/user/example_echo` 
  - [ ] Try to modify the echo server so that it caches results by connecting to 
	some caching client. Existing caching implementations can be found at `cacheclnt`,
	 `memcached`, and `kv`. Example usage can be found at `hotel` and `socialnetwork`, 
	which are two major example applications built on top of SigmaOS.
  - [ ] Try to modify the echo server so that it reads and writes to a database by
	connecting to a database proxy. Existing implementations can be found at `dbd`
	and `dbclnt`.  
  - [ ] Try to profile the echo server through `perf` package. 
