#!/bin/bash

usage() {
  echo "Usage: $0 [--push TAG] [--target target ] [--parallel]" 1>&2
}

PARALLEL=""
TAG=""
TARGET="local"
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

# tests uses hosts /tmp, which mounted in kernel container.
mkdir -p $TMP

# Make a dir to hold user proc build output
USRBIN=$(pwd)/bin/user
mkdir -p $USRBIN

# build and start db container
if [ "${TARGET}" != "aws" ]; then
    ./start-network.sh
fi

# build binaries for host
./make.sh --norace $PARALLEL linux

# Build base image
DOCKER_BUILDKIT=1 docker build --progress=plain \
  --build-arg target=$TARGET \
  --build-arg parallel=$PARALLEL \
  --build-arg tag=$TAG \
  -f build.Dockerfile \
  -t sigmabuilder .
# Default to building the sigmakernel image with user binaries
SIGMAKERNEL_TARGET="sigmakernel"
# If running on AWS, upload user bins and remove them from the base image.
if [ "${TARGET}" != "local" ]; then
  # Run the base image, which will copy the built user bins to USRBIN
  docker run -it \
    --mount type=bind,src=$USRBIN,dst=/tmp/bin \
    -e "TAG=$TAG" \
    sigmabuilder 
  ./upload.sh --tag $TAG
  # Clean up base container
  docker stop $(docker ps -aq --filter="ancestor=sigmabuilder")
  docker rm $(docker ps -aq --filter="ancestor=sigmabuilder")
  # Build the kernel image with no user binaries.
  SIGMAKERNEL_TARGET="sigmakernelclean"
fi
# Build the user image
DOCKER_BUILDKIT=1 docker build --progress=plain \
  --build-arg target=$TARGET \
  --build-arg parallel=$PARALLEL \
  --target sigmauser \
  -t sigmauser .
# Build the kernel image
DOCKER_BUILDKIT=1 docker build --progress=plain \
  --build-arg target=$TARGET \
  --build-arg parallel=$PARALLEL \
  --build-arg tag=$TAG \
  --target $SIGMAKERNEL_TARGET \
  -t sigmaos .

if ! [ -z "$TAG" ]; then
  docker tag sigmaos arielszekely/sigmaos:$TAG
  docker push arielszekely/sigmaos:$TAG
  docker tag sigmauser arielszekely/sigmauser:$TAG
  docker push arielszekely/sigmauser:$TAG
fi
