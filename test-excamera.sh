#!/bin/bash

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
echo "1. Generate Makefile"
./gen_makefile.py 1 2 16 63 > Makefile

echo "2. Clean excamera directory"
make clean
rm -f output.avi

echo "3. Initialize gg"
rm -rf .gg
gg init

echo "4. Execute 'make' to create thunks"
gg-infer make -j$(nproc)

# Refresh the executables file
rm -f $exc_dir/.gg/blobs/executables.txt
touch $exc_dir/.gg/blobs/executables.txt

for filename in $exc_dir/.gg/blobs/V*; do
  echo $(basename $filename) >> $exc_dir/.gg/blobs/executables.txt
done

# Get targets
ivfs=`ls *.ivf`
states=00000001-0.state #`ls *.state`

echo '5. Running...'
$ulambda_dir/mk-gg-ulambda-job.sh $ivfs $states | $ulambda_dir/bin/submit
