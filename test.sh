#!/bin/bash

DIR=$(dirname $0)
. $DIR/.env

VERSION=$(cat "${VERSION_FILE}")

./install.sh --from s3 --realm test-realm

for ND in :1111 :1111,:1112,:1113
do
  export NAMED=$ND
  echo "============ RUN NAMED=$ND"
  go clean -testcache
  
  #
  # test some support package
  #

  go test $1 ulambda/npcodec
  go test $1 ulambda/linuxsched
  go test $1 ulambda/perf
  
  #
  # tests without servers
  #
  go test $1 ulambda/ninep
  go test $1 ulambda/memfs
  go test $1 ulambda/pathclnt
  
  #
  # test with just named
  #
  go test $1 ulambda/reader --version=$VERSION
  go test $1 ulambda/writer --version=$VERSION
  go test $1 ulambda/stats --version=$VERSION
  go test $1 ulambda/fslib --version=$VERSION
  go test $1 ulambda/semclnt --version=$VERSION
  go test $1 ulambda/electclnt --version=$VERSION
  
  #
  # test proxy
  #
  
  go test $1 ulambda/proxy --version=$VERSION
  
  #
  # tests kernel (with 1 "fake" realm)
  #
  
  go test $1 ulambda/procclnt --version=$VERSION

  go test $1 ulambda/ux --version=$VERSION
  go test -v ulambda/fslib --version=$VERSION -path "name/ux/~ip/fslibtest/" -run ReadPerf
  
  go test $1 ulambda/s3
  go test -v ulambda/fslib --version=$VERSION -path "name/s3/~ip/9ps3/fslibtest/" -run ReadPerf
  
  go test $1 ulambda/kernel --version=$VERSION
  go test $1 ulambda/leaderclnt --version=$VERSION
  go test $1 ulambda/leadertest --version=$VERSION
  go test $1 ulambda/snapshot --version=$VERSION
  
  go test $1 ulambda/group --version=$VERSION
  go test $1 ulambda/sessclnt --version=$VERSION
  
  # dbd_test and wwwd_test requires mariadb running
  # pgrep mariadb >/dev/null && go test $1 ulambda/dbd
  # pgrep mariadb >/dev/null && go test $1 ulambda/cmd/user/wwwd
  
  
  go test $1 ulambda/mr --version=$VERSION
  go test $1 ulambda/kv --version=$VERSION
  
  # XXX broken
  # go test $1 ulambda/cmd/user/test2pc
  # go test $1 ulambda/cmd/user/test2pc2
  
  #
  # test with realms
  #
  
  go test $1 ulambda/realm --version=$VERSION
  
  # run without realm?
  # XXX needs fixing
  # go test $1 -timeout=45m ulambda/replica
done 
