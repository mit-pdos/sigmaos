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

### Exercise 1: Create, write, and read files.

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
    
To get started, see the folder `example`, which contains a Go test to
read the root directory of `named`.  The call to `test.MakeTstatePath`
starts a test instance of SigmaOS, and it embeds an `SigmaClnt`
instance, which embeds an `FsLib` object.  You can run the test as
follows:

```
$ go test -v sigmaos/example --start
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

Now extend `TestExample1` to implement the exercise.
`fslib/fslib_test` has many `fslib` tests, which may provide
inspiration.

Note that the state stored in the `named` root directory is
persistent; `named` used an `etcd` has its backend, which is a
widely-used key-value server implemented using Raft.  So, your test
should clean up after itself, because, otherwise, if you run it again,
it will fail, because your file already exists.
    
### Exercise 2: Spawn a `proc`

In this exercise, you will familiarize yourself with the `procclnt` API. In
order to do so, you will learn how to write a basic `proc`, spawn it, and wait
for it to exit. You will need to complete the following steps:
  - [ ] Create the main function for your new `proc`, by adding a new directory
    (with whatever name you choose for your `proc`) in the `cmd/` directory in
    the root of the repo, and creating a `main.go` file.
  - [ ] In the `proc`'s main file, create an `fslib.FsLib` object, and a
    `procclnt.ProcClnt` object, and put them in a `sigmaclnt.SigmaClnt` object.
  - [ ] Have the `proc` create a file, with a path of your choice, in `named`.
  - [ ] Mark the `proc` as started, to indicate to its parent that it has begun
    executing.
  - [ ] Have the `proc` log something (like "Hello World").
  - [ ] Have the `proc` mark itself  as exited, and return an exit status
    "Goodbye World" to its parent.
  - [ ] Write a test program which spawns your `proc`.
  - [ ] Have the test program wait for your `proc` to start, and look for the
    file your `proc` created in `named`. Ensure that the file is present.
  - [ ] Wait for the child `proc` to exit, and ensure that the exit status says
    "Goodbye World".

### Exercise 3: Set up a RPC server. 

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
