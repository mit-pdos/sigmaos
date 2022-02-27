#!/bin/sh

f=`mktemp`
go clean -testcache && go test -v ulambda/procclnt -run Fencer > $f 2>&1
grep stale $f > /dev/null
if [ $? -eq 0 ]; then
    rm $f
    echo OK
else
    echo $f
    echo FAIL
fi
