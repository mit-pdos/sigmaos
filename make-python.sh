#!/bin/bash
ROOT=$(pwd)
OUTPATH=./bin

mkdir -p $OUTPATH/kernel
mkdir -p $OUTPATH/user

# # Copy OpenBLAS-0.3.23
# cp /OpenBLAS-0.3.23/libopenblas64_p-r0.3.23.so $OUTPATH/kernel

# Inject custom Python lib
LIBDIR="/cpython3.11/Lib"
cp ./pylib/splib.py $LIBDIR

# Add checksum overrides for default libraries
OVERRIDEFILE="sigmaos-checksum-override"
for entry in "$LIBDIR"/*; do
  if [ -e "$entry" ]; then
    if [ -d "$entry" ]; then
      touch "$entry/$OVERRIDEFILE"
    elif [[ -f "$entry" && "$entry" == *.py ]]; then
      filename=$(basename "$entry" .py)
      touch "$LIBDIR/$filename-$OVERRIDEFILE"
    fi
  fi
done

# Copy Python executable
cp /cpython3.11/python $OUTPATH/kernel
cp -r /cpython3.11 $OUTPATH/kernel
echo "/tmp/python/Lib" > $OUTPATH/kernel/python.pth # Dummy PYTHONPATH -- not used by actual program
echo -e "home = /~~\ninclude-system-site-packages = false\nversion = 3.11.10" > $OUTPATH/kernel/pyvenv.cfg
cp /cpython3.11/python $OUTPATH/user
cp -r /cpython3.11 $OUTPATH/user
echo "/tmp/python/Lib" > $OUTPATH/user/python.pth
echo -e "home = /~~\ninclude-system-site-packages = false\nversion = 3.11.10" > $OUTPATH/user/pyvenv.cfg

# Copy and inject Python shim
gcc -Wall -fPIC -shared -o ld_fstatat.so ./ld_preload/ld_fstatat.c -ldl 
cp ld_fstatat.so $OUTPATH/kernel

# Build Python library
# gcc -I../sigmaos-local -Wall -fPIC -shared -L/usr/lib -lprotobuf-c -o clntlib.so ../sigmaos-local/pylib/clntlib.c /usr/lib/libprotobuf-c.a ../sigmaos-local/pylib/proto/proc.pb-c.c ../sigmaos-local/pylib/proto/rpc.pb-c.c ../sigmaos-local/pylib/proto/sessp.pb-c.c ../sigmaos-local/pylib/proto/sigmap.pb-c.c ../sigmaos-local/pylib/proto/spproxy.pb-c.c ../sigmaos-local/pylib/proto/timestamp.pb-c.c
# cp clntlib.so $OUTPATH/kernel

# Copy Python user processes
cp -r ./pyproc $OUTPATH/kernel

# Copy rust bins
cp ./rs/uproc-trampoline/target/release/uproc-trampoline $OUTPATH/kernel
cp ./rs/spawn-latency/target/release/spawn-latency $OUTPATH/user/spawn-latency-v$VERSION
