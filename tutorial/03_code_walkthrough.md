# 03. Code walkthrough

The SigmaOS codebase is large. This tutorial is a short walkthrough of the code
from both the client and server perspective.

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
  - `protdevclnt`: This library implements general-purpose RPCs on top of
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


## Exercise: Add a protocol message to SigmaP 

As an exercise, add a new type of `sigmap` message to SigmaOS. It can be a
no-op, or print something on the server-side. In order to do so, you'll need to
complete the following major steps (with some details left out):
  - [ ] Create a new type of `sigmap` message, and add it to the `sigmap`
    package.
  - [ ] Add an API call for the new RPC to the `fslib.FsLib` struct, and the
    lower layers it calls into, in order to invoke your RPC. You should be able
    to trace your way all the way down from the `fslib` layer to the `netclnt`
    layer.
  - [ ] Add a handler for your new `sigmap` message to `protsrv`. You should
  be able to trace your RPC's flow all the way from the `netsrv` layer to the
  `protsrv` layer.
  - [ ] Invoke your new `sigmap` message on `named` in a test, and ensure that
    it works.
