#!/bin/bash

DIR=$(dirname $0)
. $DIR/env/env.sh

./install.sh --realm testrealm # --from s3

for ND in :1111 :1111,:1112,:1113
do
  export SIGMANAMED=$ND
  echo "============ RUN SIGMANAMED=$ND"
  go clean -testcache
  
  #
  # test some support package
  #

  go test $@ sigmaos/path
  go test $@ sigmaos/serr
  go test $@ sigmaos/linuxsched
  go test $@ sigmaos/perf
  
  #
  # tests without kernel
  #
  go test $@ sigmaos/sigmap
  go test $@ sigmaos/memfs
  
  #
  # test with a kernel with just named
  #
  go test $@ sigmaos/reader
  go test $@ sigmaos/writer
  go test $@ sigmaos/stats
  go test $@ sigmaos/fslib
  go test $@ sigmaos/semclnt
  go test $@ sigmaos/electclnt
  
  #
  # test proxy
  #
  
  go test $@ sigmaos/proxy
  
  #
  # tests a full kernel (one "fake" realm)
  #
  
  go test $@ sigmaos/procclnt

  go test $@ sigmaos/ux
  go test -v sigmaos/fslib -path "name/ux/~local/fslibtest/" -run ReadPerf
  
  go test $@ sigmaos/s3
  go test -v sigmaos/fslib -path "name/s3/~local/9ps3/fslibtest/" -run ReadPerf
  
  go test $@ sigmaos/bootclnt
  go test $@ sigmaos/leaderclnt
  go test $@ sigmaos/leadertest
  go test $@ sigmaos/snapshot
  
  go test $@ sigmaos/group
  go test $@ sigmaos/sessclnt

  go test $@ sigmaos/cacheclnt

  # dbd_test and wwwd_test requires mariadb running
  pgrep mariadb >/dev/null && go test $@ sigmaos/www
  
  go test $@ sigmaos/mr
  go test $@ sigmaos/kv
  go test $@ sigmaos/hotel
  
  # XXX broken
  # go test $@ sigmaos/cmd/user/test2pc
  # go test $@ sigmaos/cmd/user/test2pc2
  
  #
  # test with realms
  #
  
  go test $@ sigmaos/realm
  
  # run without realm?
  # XXX needs fixing
  # go test $@ -timeout=45m sigmaos/replica
done 
