#!/bin/bash

# Program to run
PROGRAM="go test -v sigmaos/ckpt -start -run Geo"  # Replace with the actual path to your program
#PROGRAM ="go test -v sigmaos/ckpt -start -run CkptProc"
STOPPER="./stop.sh"
LOGS="./logs.sh > logs3.txt"
# Maximum number of iterations
MAX_ITERATIONS=200

# Time threshold in seconds
TIME_THRESHOLD=30

# Loop to run the program
for ((i=1; i<=MAX_ITERATIONS; i++))
do
    echo "Running iteration $i..."
    # Start the program in the background
    $PROGRAM &
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
    done

    # Check if the program finished successfully
    if ! kill -0 $PROGRAM_PID 2>/dev/null; then
        echo "Iteration $i completed in $SECONDS seconds."
    fi

    # Exit if the program was killed for exceeding the time limit
    if [ $SECONDS -ge $TIME_THRESHOLD ]; then
        echo "Stopping the script because the program exceeded the time threshold."
        break
    fi
    ./logs.sh > logs.txt
    # if grep -q "copyPages err resource temporarily unavailable" logs.txt; then
    #     echo "Pattern found. Exiting the script."
    #     break
    # fi
    # if grep -q "FAIL" logs.txt; then
    #     echo "FAILED!"
    #     break
    # fi
    $STOPPER
done

echo "Script completed."