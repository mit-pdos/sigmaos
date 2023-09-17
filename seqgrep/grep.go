package seqgrep

import (
	"bufio"
	"io"
	"log"
	"strings"

	"github.com/klauspost/readahead"

	"sigmaos/proc"
	"sigmaos/mr"
	"sigmaos/perf"
)

//func grepline1(n int, line string) {
//	re := regexp.MustCompile("[^a-zA-Z0-9_\\s]+")
//	sanitized := strings.ToLower(re.ReplaceAllString(line, " "))
//	for _, word := range strings.Fields(sanitized) {
//		if word == "scala" {
//			fmt.Printf("%d:%s\n", n, word)
//		}
//	}
//}

func grepline(n int, line string, sbc *mr.ScanByteCounter) int {
	scanner := bufio.NewScanner(strings.NewReader(line))
	scanner.Split(sbc.ScanWords)
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

func Grep(pcfg *proc.ProcEnv, rdr io.Reader) int {
	p, err := perf.NewPerf(pcfg, perf.SEQGREP)
	if err != nil {
		log.Fatalf("NewPerf err %v\n", err)
	}
	sbc := mr.NewScanByteCounter(p)
	sz := 8 * (1 << 20)
	ra, err := readahead.NewReaderSize(rdr, 4, sz)
	if err != nil {
		log.Fatalf("readahead err %v\n", err)
	}
	scanner := bufio.NewScanner(ra)
	buf := make([]byte, 0, sz)
	scanner.Buffer(buf, cap(buf))
	n := 1
	cnt := 0
	for scanner.Scan() {
		l := scanner.Text()
		cnt += grepline(n, l, sbc)
		n += 1
	}
	return cnt
}
