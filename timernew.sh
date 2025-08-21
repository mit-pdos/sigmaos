#!/bin/bash
# Number of runs
num_runs=$1
# Initialize an associative array to store cumulative timestamp values (in ms)
declare -A total_times
./stop.sh > /dev/null 2>&1
# Function to convert timestamp to milliseconds
timestamp_to_ms() {
    # Use `date` to convert H:M:S(.ms) to epoch time with milliseconds
    date -u -d "1970-01-01T$1" +%s%3N
}
> restore_time.txt
# Initialize the array with zeros
for ((i = 0; i < ${#patterns[@]}; i++)); do
    total_times[$i]=0
done
# Run the tests and capture the logs with timestamps
# echo "Running tests and capturing logs..."
total_faults=0
for ((run = 1; run <= num_runs; run++)); do
    echo "Running iteration $run..."
    go test -v sigmaos/ckpt -start -run Geo >stdlog.txt
    # # Extract timestamps for the desired log lines
    declare -A times

    # # Define the patterns to search for
    patterns=(
        "Spawn from checkpoint"
        "restoreProc: Register "
        "restoreProc: Registered"
        "CKPT readCheckpoint"
        "Done readCheckpoint"
        "Invoke restore"
        "restoreProc: Restore err"
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
            #break
        fi
    done < stdlog.txt

    ./logs.sh > logs.txt

    times[0]=$first_timestamp  # Store the first timestamp from stdout

    #Error checker
    FILE="logs.txt"
    # if grep -Eq "copyPages err|Read err|Recvmsg err|file exists" logs.txt; then
    #      echo "!!!!!BAD PATTERN FOUND!!!!!"
    #      grep -Eo "copyPages err|Read err|Poll err|Recvmsg err|  file exists" logs.txt
    #      ./stop.sh
    #      break
    #  fi


    # Extract last occurrence of npages(number)
    last_npages=$(grep -o 'npages([0-9]\+)' "$FILE" | tail -n 1 | grep -o '[0-9]\+')
    total_faults=$((total_faults + last_npages))
    # Output result
    # if [ -n "$last_npages" ]; then
    # echo "Last npages value: $last_npages"
    # else
    # echo "No npages(number) found in the file."
    # fi
        
    for pattern in "${patterns[@]:1:${#patterns[@]}-2}"; do
        found=false
        pat="Recvmsg err"
        while IFS= read -r line; do
            
            if [[ "$line" == *"$pattern"* ]]; then
                # Extract the timestamp and store it
                timestamp=$(echo "$line" | awk '{print $1}')  # Extract timestamp from logs.txt
                times[$line_num]=$timestamp
                
         #       break
            fi
            if [[ "$line" == *"$pat"* ]]; then
                echo "FOUND BAD"
                found=true
            fi
        
        done < logs.txt  # Make sure to reference logs.txt
       # echo "line $line_num $pattern"
        line_num=$((line_num + 1))
        
        if $found; then
            echo "not found $pattern"
            break
        fi
    done    

    times[$line_num]=$last_timestamp
    # Ensure we have all the required timestamps
    # if [ "${#times[@]}" -ne ${#patterns[@]} ]; then
    #     echo "Error: Could not find all required patterns in logs.txt ${#times[@]} <${#patterns[@]}"
    #     ./stop.sh > /dev/null 2>&1
    #     exit 1
    # fi
    
    for ((i = 0; i < ${#patterns[@]}; i++)); do
            if [[ -n "${times[$i]}" ]]; then
            
                echo "$i ${patterns[$i]} ${times[$i]}"
                ms=$(timestamp_to_ms "${times[$i]}")
                
                total_times[$i]=$((total_times[$i] + ms))
            fi
        done
    # if (( $(echo "$(timestamp_to_ms "${times[2]}")> $(timestamp_to_ms "${times[5]}")" | bc -l) )); then
    #             echo "FOUND!"
    #             break
    #         fi
    diff=$(echo "$(timestamp_to_ms "${times[6]}") - $(timestamp_to_ms "${times[5]}")" | bc -l)
    echo "$diff" >> restore_time.txt
    if (( $(echo "$diff > 80" | bc -l) )); then
        echo "FOUND! $diff"
        for ((i = 0; i < ${#patterns[@]}; i++)); do
            #avg_ms=$((total_times[$i] / num_runs))
            total_times[$i]=$(timestamp_to_ms "${times[$i]}")
            
        done

        echo "Timestamps"
        for ((i = 1; i < ${#patterns[@]}; i++)); do
            t1=${total_times[0]}
            t2=${total_times[$i]}
            diff=$((t2 - t1))
            echo "${patterns[$i]} offset: ${diff} ms"
        done
        break  # only valid inside a loop
    fi
    ./stop.sh > /dev/null 2>&1
done
average=$(echo "scale=2; $total_faults / $num_runs" | bc)
echo "Average npages $average"

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
# echo "Time Differences (in ms):"
# for ((i = 1; i < ${#patterns[@]}; i++)); do
#     t1=$(timestamp_to_ms "${times[$((i - 1))]}")
#     t2=$(timestamp_to_ms "${times[$i]}")
#     diff=$((t2 - t1))
    
#     echo "${times[$((i - 1))]} ${times[$i]} ${patterns[$((i - 1))]} -> ${patterns[$i]}: ${diff} ms"
# done
