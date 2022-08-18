package seqwc

import (
	"bufio"
	"io"
	"log"
	"strings"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/klauspost/readahead"

	db "ulambda/debug"
	"ulambda/fslib"
	"ulambda/mr"
	np "ulambda/ninep"
	"ulambda/test"
)

type Tdata map[string]uint64

func wcline(n int, line string, data Tdata) int {
	scanner := bufio.NewScanner(strings.NewReader(line))
	scanner.Split(mr.ScanWords)
	cnt := 0
	for scanner.Scan() {
		w := scanner.Text()
		if _, ok := data[w]; !ok {
			data[w] = uint64(0)
		}
		data[w] += 1
		cnt++
	}
	if err := scanner.Err(); err != nil {
		log.Fatalf("scanner err %v\n", err)
	}
	return cnt
}

func wcFile(rdr io.Reader, data Tdata) int {
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
		cnt += wcline(n, l, data)
		n += 1
	}
	return cnt
}

func Wc(fsl *fslib.FsLib, dir string) (int, error) {
	sts, err := fsl.GetDir(dir)
	if err != nil {
		return 0, err
	}
	data := make(Tdata)
	n := 0
	start := time.Now()
	nbytes := np.Tlength(0)
	for _, st := range sts {
		nbytes += st.Length
		r, err := fsl.OpenReader(dir + "/" + st.Name)
		if err != nil {
			return 0, err
		}
		rdr := bufio.NewReader(r)
		m := wcFile(rdr, data)
		// log.Printf("%v: %d\n", st.Name, m)
		n += m
	}
	//for k, v := range data {
	//	fmt.Printf("%s %d\n", k, v)
	//}
	ms := time.Since(start).Milliseconds()
	db.DPrintf(db.ALWAYS, "Wc %s took %vms (%s)", humanize.Bytes(uint64(nbytes)), ms, test.TputStr(nbytes, ms))
	return n, nil
}
