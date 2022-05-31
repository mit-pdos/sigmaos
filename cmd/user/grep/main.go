package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"

	"ulambda/mr"
)

func grep1(n int, line string) {
	re := regexp.MustCompile("[^a-zA-Z0-9_\\s]+")
	sanitized := strings.ToLower(re.ReplaceAllString(line, " "))
	for _, word := range strings.Fields(sanitized) {
		if word == "scala" {
			fmt.Printf("%d:%s\n", n, word)
		}
	}
}

func grep(n int, line string) {
	scanner := bufio.NewScanner(strings.NewReader(line))
	scanner.Split(mr.ScanWords)
	for scanner.Scan() {
		w := scanner.Text()
		if w == "Scala" {
			fmt.Printf("%d:%s\n", n, w)
		}
	}
	if err := scanner.Err(); err != nil {
		log.Fatalf("scanner err %v\n", err)
	}
}

func main() {
	f, err := os.Open(os.Args[1])
	if err != nil {
		log.Fatal("cannot open %s\n", os.Args[1])
	}
	rdr := bufio.NewReader(f)
	scanner := bufio.NewScanner(rdr)
	n := 1
	for scanner.Scan() {
		l := scanner.Text()
		grep(n, l)
		n += 1
	}
}
