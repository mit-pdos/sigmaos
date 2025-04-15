#!/bin/bash
num_runs=$1
# Initialize an associative array to store cumulative timestamp values (in ms)
declare -A total_times

# Function to convert timestamp to milliseconds
timestamp_to_ms() {
    # Use `date` to convert H:M:S(.ms) to epoch time with milliseconds
    date -u -d "1970-01-01T$1" +%s%3N
}
# Initialize the array with zeros
for ((i = 0; i < ${#patterns[@]}; i++)); do
    total_times[$i]=0
done


for ((run = 1; run <= num_runs; run++)); do
    echo "Running iteration $run..."
    go test -v sigmaos/ckpt -start -run Geo >stdlog.txt
    # # Extract timestamps for the desired log lines
    declare -A times

    # # Define the patterns to search for
    patterns=(
        "Spawn proc"
        "GeoSrv start"
        "I'm here"
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
            #break
        fi
    done < stdlog.txt


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
        done < /tmp/sigmaos-perf/log-proc.txt  # Make sure to reference logs.txt
        
    done    


    for ((i = 0; i < ${#patterns[@]}; i++)); do
                if [[ -n "${times[$i]}" ]]; then
                    ms=$(timestamp_to_ms "${times[$i]}")
#                    zero=$(timestamp_to_ms "${times[0]}")
                    total_times[$i]=$((total_times[$i] + ms))
                    # diff=$((ms - zero))
                    # echo "${times[$i]} ${patterns[$i]} offset: ${diff} ms"
                fi
            done
        ./stop.sh > /dev/null 2>&1
done

for ((i = 0; i < ${#patterns[@]}; i++)); do
    #avg_ms=$((total_times[$i] / num_runs))
    total_times[$i]=$((total_times[$i] / num_runs))
    
done

echo "Timestamps"
for ((i = 1; i < ${#patterns[@]}; i++)); do
    t1=${total_times[0]}
    t2=${total_times[$i]}
    diff=$((t2 - t1))
    echo "${patterns[$i]} offset: ${diff} ms"
done
# echo "Timestamps"
# for ((i = 1; i < ${#patterns[@]}; i++)); do
#     t1=$(timestamp_to_ms "${times[0]}")
#     t2=$(timestamp_to_ms "${times[$i]}")
#     diff=$((t2 - t1))
#     echo "${times[$i]} ${patterns[$i]} offset: ${diff} ms"
# done
# echo "Time Differences (in ms):"
# for ((i = 1; i < ${#patterns[@]}; i++)); do
#     t1=$(timestamp_to_ms "${times[$((i - 1))]}")
#     t2=$(timestamp_to_ms "${times[$i]}")
#     diff=$((t2 - t1))
    
#     echo "${times[$((i - 1))]} ${times[$i]} ${patterns[$((i - 1))]} -> ${patterns[$i]}: ${diff} ms"
# done
# ./stop.sh > /dev/null 2>&1