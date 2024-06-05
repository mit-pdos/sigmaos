# 04. Code walkthrough

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
    has handlers for each of the `sigmap` messages, and deals with SigmaOS
    features like leased files, and watches.

### Exercise 1: Describe SigmaOS librarly layers

The following exercises will help you get familiar with the core of
the SigmaOS codebase.  In this exercise, you will draw a diagram of
how the SigmaOS client and server libraries stack together, and
explain what their purposes are. You should hit the following
waypoints in your traversal:
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

