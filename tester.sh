#!/bin/bash

# Program to run
PROGRAM="go test -v sigmaos/ckpt -start -run CkptProc"  # Replace with the actual path to your program
# Maximum number of iterations
MAX_ITERATIONS=200

# Time threshold in seconds
TIME_THRESHOLD=45

# Loop to run the program
for ((i=1; i<=MAX_ITERATIONS; i++))
do
    ./stop.sh --parallel --nopurge --skipdb
    echo "Running iteration $i..."

    # Start the program in the background
    $PROGRAM  2>&1 | tee /tmp/out-xxxx &
    PROGRAM_PID=$!

    # Wait for the program to complete with a timeout
    SECONDS=0
    while kill -0 $PROGRAM_PID 2>/dev/null; do
        if [ $SECONDS -ge $TIME_THRESHOLD ]; then
            echo "Program is taking too long (>$TIME_THRESHOLD seconds). Killing it."
            kill -9 $PROGRAM_PID 2>/dev/null
            break
        fi
        sleep 1
        echo $SECONDS
    done

    # Check if the program finished successfully
    if ! kill -0 $PROGRAM_PID 2>/dev/null; then
        echo "Iteration $i completed in $SECONDS seconds."
    fi

    if grep -q "Shutdown" /tmp/out-xxxx; then
      continue
    fi

    # Exit if the program was killed for exceeding the time limit
    if [ $SECONDS -ge $TIME_THRESHOLD ]; then
        echo "Stopping the script because the program exceeded the time threshold."
        break
    fi
done

echo "Script completed."
