#!/bin/bash

memfs_dir=/mnt/9p/fs/gg

if [ ! -d "/mnt/9p/fs" ]
then
  echo "9p not mounted!"
  exit 1
fi

echo "1. Set up memfs dirs..."
mkdir -p $memfs_dir
mkdir -p $memfs_dir/results

exc_dir=$HOME/gg/examples/excamera
ulambda_dir=$HOME/ulambda

# Set up env vars
cd $HOME/gg
. ./.environment

# Remove old local environments
rm -rf /tmp/ulambda/*

# Remove cache
rm -rf $HOME/.cache/gg

cd $exc_dir

# Set up the thunks for GG
echo "2. Generate Makefile"
./gen_makefile.py 1 6 16 63 > Makefile

echo "3. Clean excamera directory"
make clean
rm -f output.avi

echo "4. Initialize gg"
rm -rf .gg
gg init

echo "5. Execute 'make' to create thunks"
gg-infer make -j$(nproc)

# Refresh the executables file
rm -f $exc_dir/.gg/blobs/executables.txt
touch $exc_dir/.gg/blobs/executables.txt

for filename in $exc_dir/.gg/blobs/V*; do
  echo $(basename $filename) >> $exc_dir/.gg/blobs/executables.txt
done

# Get targets
ivfs=`ls *.ivf`
states=`ls *.state`
targets="${ivfs} ${states}"

echo '6. Copying to memfs...'
cp -r ./.gg $memfs_dir
cp ./$targets $memfs_dir

echo '7. Running...'
$ulambda_dir/mk-gg-ulambda-job.sh $targets | $ulambda_dir/bin/submit
