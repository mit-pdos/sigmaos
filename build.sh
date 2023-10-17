#!/bin/bash

usage() {
  echo "Usage: $0 [--push TAG] [--target TARGET] [--userbin USERBIN] [--parallel]" 1>&2
}

PARALLEL=""
TAG=""
TARGET="local"
USERBIN="all"
while [[ "$#" -gt 0 ]]; do
  case "$1" in
  --parallel)
    shift
    PARALLEL="--parallel"
    ;;
  --push)
    shift
    TAG="$1"
    shift
    ;;
  --target)
    shift
    TARGET="$1"
    shift
    ;;
  --userbin)
    shift
    USERBIN="$1"
    shift
    ;;
  -help)
    usage
    exit 0
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

if [[ "$TAG" != "" && "$TARGET" == "local" ]] || [[ "$TAG" == "" && "$TARGET" != "local" ]] ; then
  echo "Must run with either --push set and --target=aws, or --target=local and without --push"
  exit 1
fi

TMP=/tmp/sigmaos
BUILD_LOG=/tmp/sigmaos-build

# tests uses hosts /tmp, which mounted in kernel container.
mkdir -p $TMP
mkdir -p $BUILD_LOG

# Make a dir to hold user proc build output
USRBIN=$(pwd)/bin/user
mkdir -p $USRBIN

# build and start db container
if [ "${TARGET}" != "aws" ]; then
    ./start-network.sh
fi

# build binaries for host
./make.sh --norace $PARALLEL linux

BUILD_ARGS="--progress=plain \
  --build-arg target=$TARGET \
  --build-arg userbin=$USERBIN \
  --build-arg parallel=$PARALLEL \
  --build-arg tag='$TAG'"

builders="build-kernel build-user-rust build-user"

njobs=1
if ! [ -z "$PARALLEL" ]; then
  # Optionally build the docker images in parallel.
  njobs=$(echo $builders | wc -w)
fi

build_builders="parallel -j$njobs \"DOCKER_BUILDKIT=1 docker build $BUILD_ARGS -f {}.Dockerfile -t sigma-{} . 2>&1 | tee $BUILD_LOG/sigmaos-{}.out\" ::: $builders"

printf "\nBuilding Docker builders\n$build_builders\n\n"
echo "========== Start Docker builders build =========="
eval $build_builders
echo "========== Done building Docker builders =========="


# Now, prepare to build final containers which will actually run.
targets="sigmauser sigmaos"
if [ "${TARGET}" == "local" ]; then
  targets="sigmauser sigmaos-with-userbin"
fi
build_targets="parallel -j$njobs \"DOCKER_BUILDKIT=1 docker build $BUILD_ARGS -f Dockerfile --target {} -t {} . 2>&1 | tee $BUILD_LOG/{}.out\" ::: $targets"

printf "\nBuilding Docker targets\n$build_targets\n\n"
echo "========== Start Docker targets build =========="
eval $build_targets
echo "========== Done building Docker targets =========="

if [ "${TARGET}" == "local" ]; then
  # If developing locally, rename the sigmaos image which includes binaries to
  # be the default sigmaos image.
  docker tag sigmaos-with-userbin sigmaos
else
  echo "========== Copying user bins to $USRBIN =========="
  # If not developing locally, push user binaries to S3
  # Copy user bins to USRBIN
  docker run --rm -it \
    --mount type=bind,src=$USRBIN,dst=/tmp/bin \
    -e "TAG=$TAG" \
    sigma-build-user
  echo "========== Copying user rust bins to $USRBIN =========="
  docker run --rm -it \
    --mount type=bind,src=$USRBIN,dst=/tmp/bin \
    -e "TAG=$TAG" \
    sigma-build-user-rust
  # Upload the user bins to S3
  echo "========== Pushing user bins to aws =========="
  ./upload.sh --tag $TAG --profile sigmaos
  echo "========== Done pushing user bins to aws =========="
fi

if ! [ -z "$TAG" ]; then
  docker tag sigmaos arielszekely/sigmaos:$TAG
  docker push arielszekely/sigmaos:$TAG
  docker tag sigmauser arielszekely/sigmauser:$TAG
  docker push arielszekely/sigmauser:$TAG
fi
