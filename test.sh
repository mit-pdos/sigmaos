#!/bin/bash

export NAMED=127.0.0.1:1111

go test ulambda/memfs
go test ulambda/fsclnt
go test ulambda/fslib
go test ulambda/schedd
(cd nps3d; go test)
(cd mr; go test)
(cd kv; go test)
