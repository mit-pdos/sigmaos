pairs=()

while IFS= read -r line; do
     if [[ "$line" =~ faulting\ copy\ ([0-9a-fA-Fx]+)-\> ]]; then
        addr="${BASH_REMATCH[1]}"
        if [[ "$line" =~ len:\ ([0-9a-fA-Fx]+), ]]; then
            len="${BASH_REMATCH[1]}"
            pairs+=("{0x${addr}, ${len}}")
        fi
    fi
done < logs.txt
#echo $pairs
Print Go-style 2D array of uintptr pairs
echo "[] [2]uintptr{"
for pair in "${pairs[@]}"; do
    echo "    $pair,"
done
echo "}"