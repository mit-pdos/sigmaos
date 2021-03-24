#!/bin/bash

export NAMED=127.0.0.1:1111

go test $1 ulambda/memfs
go test $1 ulambda/fsclnt
go test $1 ulambda/fslib
(cd nps3; go test $1)
go test $1 ulambda/schedd
(cd mr; go test $1)
(cd kv; go test $1)
