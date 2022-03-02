#!/bin/sh

echo "=== RUN Fences"

f=`mktemp`
go clean -testcache && go test -v ulambda/procclnt -run Fencer > $f 2>&1
grep stale $f > /dev/null
if [ $? -eq 0 ]; then
    rm $f
    echo "--- PASS: Fences"
else
    echo $f
    echo "--- FAIL: Fences"
fi
