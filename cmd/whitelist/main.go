package main

import (
	"bufio"
	"fmt"
	"os"
	"sort"
	"strings"
)

// Parse output of strace -c to print just the system calls

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintf(os.Stderr, "Usage: %v <strace file>\n", os.Args[0])
		os.Exit(1)
	}
	file, err := os.Open(os.Args[1])
	if err != nil {
		return
	}
	defer file.Close()

	syscalls := []string{}

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if fields[0] == "%" || fields[0] == "100.00" ||
			strings.HasPrefix(fields[0], "-----") {
			continue
		}
		syscalls = append(syscalls, fields[len(fields)-1])
	}
	sort.Strings(syscalls)
	fmt.Println("var whitelist = []string{")
	for _, s := range syscalls {
		fmt.Println("\t\"" + s + "\",")
	}
	fmt.Println("}")
}
