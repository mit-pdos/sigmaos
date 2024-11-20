#!/bin/bash

#
# Start npproxy and mount root named at /mnt/9p. Optionally boot a
# rootrealm named.
#

usage() {
  echo "Usage: $0 --boot <IPaddr>"  1>&2
}

BOOT=""
while [ $# -ne 1 ]; do
    case "$1" in
        --boot)
            shift
            BOOT="--boot"
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

if [[ "$BOOT" == "--boot" ]] ; then
    (cd npproxysrv; ../start-kernel.sh --pull local-build --boot all --usedialproxy sigma-named )
    (cd npproxysrv; ../start-kernel.sh --pull local-build --boot spproxyd --usedialproxy sigma-scd )
fi

./bin/npproxy/npproxyd $1 &

sleep 1

sudo mount -t 9p -o trans=tcp,aname=`whoami`,uname=`whoami`,port=1110 $1 /mnt/9p
