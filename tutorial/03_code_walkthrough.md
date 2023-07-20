# 03. Code walkthrough

The SigmaOS codebase is large. This tutorial is a short walkthrough of the code
from both the client and server perspective. By the end of this tutorial you
should be able to use the SigmaOS API to manipulate files and directories,
create and manage `procs`, and understand the different layers of SigmaOS
clients and servers, and how the pacakges that implement them fit together.

XXX TODO: insert diagram.

## Client-side libraries

This section describes the SigmaOS client-side interface and the libraries that
implement it. The following list of libraries is not exhaustive (many of them
call into other libraries), but most SigmaOS clients and `procs` will use only
a subset of these libraries:
  - `fslib`: This is the main library that all clients use to interact in
    SigmaOS. It defines common file-system operations (like `Open`, `Write`,
    `Read`, `Close`) as well as operations for fault tolerance (like
    `Watch`es). All other libraries are built on `fslib`.
  - `procclnt`: This library designs and implements the `proc` API. `proc`s,
    benchmarks, and tests use the `procclnt` API to `Spawn`, `Evict`, and
    `Wait` for `proc`s.
  - `sigmaclnt`: This library unifies the `fslib` and `procclnt` structures
    into a single interface. It is mostly for convenience.
  - `semclnt`: This library defines the SigmaOS equivalent of semaphores.
  - `epochclnt`: This library lets users define epochs. It is useful to
    implement leader election.
  - `electclnt`: This library implements a simple leader election protocol on
    top of `named`.
  - `leaderclnt`: This library uses `electclnt` and `epochclnt` to implement
    leader election, and fence directories and services with the leader's
    epoch.
  - `rpcclnt`: This library implements general-purpose RPCs on top of
    SigmaOS. RPCs have fewer guarantees (e.g., no in-order delivery guarantees)
    but are more expressive and more performant than general SigmaOS
    operations.

The following libraries make up the "base" on which the SigmaOS client-side is
built:
  - `sessclnt`: This library implements SigmaOS's session layer which
    guarantees, among other things, in-order message delivery, transparent
    failover, and exactly-once RPCs under network partition.
  - `netclnt`: This library abstracts TCP connections from the client
    perspective, and is used to send messages from the client to the server,
    and deliver responses.

## Server-side libraries

This section describes the SigmaOS server-side libraries. It is not an
exhaustive list, but it contains some of the more interesting design points,
and libraries which may be useful for future projects based on SigmaOS.
  - `replraft`: This layer wraps etcd's Raft library, and is used to replicate
    SigmaOS servers.
  - `netsrv`: This layer abstracts TCP connections from the server perspective,
    and is used to receive messages from the client and send responses back.
  - `sesssrv`: This library contains the server-side session protocol, which
    collaborates with `sessclnt` to guarantee in-order message delivery,
    exactly-once RPCs under network partition, and collaborates with `replraft`
    to transparently replicate `sigmap` messages.
  - `sessstatesrv`: This library contains server-side data structures needed to
    represent, track, and manage the lifecycle of sessions.
  - `protsrv`: This library implements a generic SigmaOS protocol server. It
    has handlers for each of teh `sigmap` messages, and deals with SigmaOS
    features like versions, ephemeral files, and watches.

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

### Exercise 2: Describe SigmaOS librarly layers

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

### Exercise 3: Add a protocol message to SigmaP 

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

### Exercise 4: Spawn a `proc`

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

### Exercise 5: Set up a RPC server. 
In this exercise, you will familiarize with the application layer APIs of SigmOS, 
specifically `rpcclnt` and `sigmasrv`. In order to do so, you will learn 
how to set up a basic RPC server, and explore existing utilities that provide 
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
