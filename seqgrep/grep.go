package seqgrep

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"regexp"
	"strings"

	"github.com/klauspost/readahead"

	"ulambda/mr"
	"ulambda/test"
)

func grepline1(n int, line string) {
	re := regexp.MustCompile("[^a-zA-Z0-9_\\s]+")
	sanitized := strings.ToLower(re.ReplaceAllString(line, " "))
	for _, word := range strings.Fields(sanitized) {
		if word == "scala" {
			fmt.Printf("%d:%s\n", n, word)
		}
	}
}

func grepline(n int, line string) int {
	scanner := bufio.NewScanner(strings.NewReader(line))
	scanner.Split(mr.ScanWords)
	cnt := 0
	for scanner.Scan() {
		w := scanner.Text()
		if w == "Scala" {
			// fmt.Printf("%d:%s\n", n, w)
			cnt += 1
		}
	}
	if err := scanner.Err(); err != nil {
		log.Fatalf("scanner err %v\n", err)
	}
	return cnt
}

func Grep(rdr io.Reader) int {
	ra, err := readahead.NewReaderSize(rdr, 4, test.BUFSZ)
	if err != nil {
		log.Fatalf("readahead err %v\n", err)
	}
	scanner := bufio.NewScanner(ra)
	buf := make([]byte, 0, test.BUFSZ)
	scanner.Buffer(buf, cap(buf))
	n := 1
	cnt := 0
	for scanner.Scan() {
		l := scanner.Text()
		cnt += grepline(n, l)
		n += 1
	}
	return cnt
}
