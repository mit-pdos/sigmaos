

usage() {
  echo "Usage: $0 [--realm realm] " 1>&2
}

REALM=""

while [[ "$#" -gt 0 ]]; do
    case "$1" in
        --realm)
            shift
            REALM="$1"
            shift
            ;;
        *)
            usage
            exit 1
    esac
done

if [ $# -gt 0 ]; then
    usage
    exit 1
fi

if [[ "$REALM" != "" ]] ; then
    ./test-in-docker.sh --pkg namesrv/fsetcd --run Dump --args "--realm \"$REALM\""
else
    ./test-in-docker.sh --pkg namesrv/fsetcd --run Dump
fi


