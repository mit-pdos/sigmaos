#!/bin/bash

# Run the tests and capture the logs with timestamps
# echo "Running tests and capturing logs..."
go test -v sigmaos/ckpt -start -run Geo >stdlog.txt
# # Extract timestamps for the desired log lines
declare -A times

# # Define the patterns to search for
patterns=(
    "Spawn from checkpoint"
    "readCheckpoint"
    "Done readCheckpoint"
    "restoreProc: Register"
    "restoreProc: Registered"
    "Invoke restore"
    "restoreProc: Restore err"
    "LAZYPAGESSRV_FAULT page fault"
    "restored proc is running"
    "sendConn: sent"
    "TEST Started"
)

# Initialize line counter
line_num=1
# Capture stdout and extract timestamp for the first pattern
first_timestamp=""
last_timetamp=""
while IFS= read -r line; do
    #echo 
    if [[ "$line" == *"${patterns[0]}"* ]]; then
        
        first_timestamp=$(echo "$line" | awk '{print $1}')  # Extract timestamp
        #break
    fi
    if [[ "$line" == *"${patterns[-1]}"* ]]; then
        
        last_timestamp=$(echo "$line" | awk '{print $1}')  # Extract timestamp
        echo "$line"
        #break
    fi
done < stdlog.txt
./logs.sh > logs.txt

times[0]=$first_timestamp  # Store the first timestamp from stdout


    
for pattern in "${patterns[@]}"; do
    found=false
    while IFS= read -r line; do
        
        if [[ "$line" == *"$pattern"* ]]; then
            # Extract the timestamp and store it
            timestamp=$(echo "$line" | awk '{print $1}')  # Extract timestamp from logs.txt
            times[$line_num]=$timestamp
            line_num=$((line_num + 1))
            found=true
            break
        fi
    done < logs.txt  # Make sure to reference logs.txt
    if ! $found; then
        echo "Pattern not found: $pattern"
    fi
    
done    

times[$line_num]=$last_timestamp
# Ensure we have all the required timestamps
if [ "${#times[@]}" -ne ${#patterns[@]} ]; then
    echo "Error: Could not find all required patterns in logs.txt ${#times[@]} <${#patterns[@]}"
    ./stop.sh > /dev/null 2>&1
    exit 1
fi

# Function to convert timestamp to milliseconds
timestamp_to_ms() {
    # Use `date` to convert H:M:S(.ms) to epoch time with milliseconds
    date -u -d "1970-01-01T$1" +%s%3N
}
# Calculate and display the time differences

echo "Timestamps"
for ((i = 1; i < ${#patterns[@]}; i++)); do
    t1=$(timestamp_to_ms "${times[0]}")
    t2=$(timestamp_to_ms "${times[$i]}")
    diff=$((t2 - t1))
    echo "${times[$i]} ${patterns[$i]} offset: ${diff} ms"
done
echo "Time Differences (in ms):"
for ((i = 1; i < ${#patterns[@]}; i++)); do
    t1=$(timestamp_to_ms "${times[$((i - 1))]}")
    t2=$(timestamp_to_ms "${times[$i]}")
    diff=$((t2 - t1))
    
    echo "${times[$((i - 1))]} ${times[$i]} ${patterns[$((i - 1))]} -> ${patterns[$i]}: ${diff} ms"
done
./stop.sh > /dev/null 2>&1