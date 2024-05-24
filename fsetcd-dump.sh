

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
    go clean -testcache && go test -v sigmaos/fsetcd -run Dump --realm "$REALM"
else
    go clean -testcache && go test -v sigmaos/fsetcd -run Dump
fi


