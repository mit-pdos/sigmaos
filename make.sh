#!/bin/bash

usage() {
  echo "Usage: $0 [--norace] [--vet] [--parallel] [--gopath GO] [--target local|remote] [--version VERSION] [--userbin USERBIN] kernel|user|npproxy" 1>&2
}

RACE="-race"
CMD="build"
TARGET="local"
VERSION="1.0"
USERBIN="all"
GO="go"
PARALLEL=""
WHAT=""
BINS=""
while [[ "$#" -gt 0 ]]; do
  case "$1" in
  --norace)
    shift
    RACE=""
    ;;
  --vet)
    shift
    CMD="vet"
    ;;
  --gopath)
    shift
    GO="$1"
    shift
    ;;
  --target)
    shift
    TARGET="$1"
    shift
    ;;
  --version)
    shift
    VERSION="$1"
    shift
    ;;
  --userbin)
    shift
    USERBIN="$1"
    shift
    ;;
  --parallel)
    shift
    PARALLEL="--parallel"
    ;;
  --bins)
    shift
    BINS="$1"
    shift
    ;;
  -help)
    usage
    exit 0
    ;;
  kernel|user|linux|npproxy)
    WHAT=$1
    shift
    ;;
  *)
   echo "unexpected argument $1"
   usage
   exit 1
  esac
done

if [ $# -gt 0 ]; then
    usage
    exit 1
fi

if [[ "$TARGET" != "local" ]] && [[ "$TARGET" != "remote" ]]; then
  echo "Error! Build target must be either \"local\" or \"remote\""
  exit 1
fi

echo $WHAT

OUTPATH=bin

LDF="-X sigmaos/sigmap.Target=$TARGET -X sigmaos/sigmap.Version=$VERSION -s -w"

if [[ $WHAT == "kernel" ]]; then
    mkdir -p $OUTPATH/kernel
    mkdir -p $OUTPATH/linux
    WHAT="kernel linux"
    # Clear version string, which only applies to user procs
    VERSION=""
elif [[ $WHAT == "user" ]]; then
    mkdir -p $OUTPATH/user
    # Prepend version string prefix "-v" for user procs
    VERSION="-v$VERSION"
    $GO build -ldflags="$LDF" $RACE -buildmode=plugin -o $OUTPATH/user/hotel-geod-plugin$VERSION cmd/user/hotel-geod/main.go
elif [[ $WHAT == "npproxy" ]]; then
    mkdir -p $OUTPATH/npproxy
    # Clear version string, which only applies to user procs
    VERSION=""
else
    mkdir -p $OUTPATH/linux
    WHAT="linux"
    # Clear version string, which only applies to user procs
    VERSION=""
fi

for k in $WHAT; do
  echo "Building $k components $VERSION"
  FILES=`ls cmd/$k`
   if [[ "$k" == "user" ]] && ! [[ "$USERBIN" == "all" ]] ; then
     FILES="$(echo "$USERBIN" | tr "," " ")"
     echo "Only building userbin $USERBIN files $FILES"
   fi
  if [ -z "$PARALLEL" ]; then
    for f in $FILES;  do
      if [ $CMD == "vet" ]; then
        echo "$GO vet cmd/$k/$f/main.go"
        $GO vet cmd/$k/$f/main.go
      else
        build="$GO build -ldflags=\"$LDF\" $RACE -o $OUTPATH/$k/$f$VERSION cmd/$k/$f/main.go"
        echo $build
        eval "$build"
        # Bail out early on build error
        export EXIT_STATUS=$?
        if [ $EXIT_STATUS  -ne 0 ]; then
          exit $EXIT_STATUS
        fi
      fi
    done
  else
    # If building in parallel, build with (n - 1) threads.
    njobs=$(nproc)
    njobs="$(($njobs-1))"
    build="parallel -j$njobs $GO \"build -ldflags='$LDF' $RACE -o $OUTPATH/$k/{}$VERSION cmd/$k/{}/main.go\" ::: $FILES"
    echo $build
    eval $build
    # Bail out early on build error
    export EXIT_STATUS=$?
    if [ $EXIT_STATUS  -ne 0 ]; then
      exit $EXIT_STATUS
    fi
  fi
done
