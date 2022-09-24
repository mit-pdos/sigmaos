package seqwc

import (
	"bufio"
	"fmt"
	// "encoding/json"
	"io"
	"log"
	"strings"
	"time"

	"github.com/dustin/go-humanize"

	db "sigmaos/debug"
	"sigmaos/fslib"
	"sigmaos/mr"
	np "sigmaos/ninep"
	"sigmaos/test"
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
		// kv := &mr.KeyValue{scanner.Text(), "1"}
		// _, err := json.Marshal(kv)
		// if err != nil {
		// 	db.DFatalf("json")
		// }
		data[w] += 1
		cnt++
	}
	if err := scanner.Err(); err != nil {
		log.Fatalf("scanner err %v\n", err)
	}
	return cnt
}

func wcFile(rdr io.Reader, data Tdata) int {
	scanner := bufio.NewScanner(rdr)
	buf := make([]byte, 0, 8*np.MBYTE)
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

func WcData(fsl *fslib.FsLib, dir string, data Tdata) (int, np.Tlength, error) {
	sts, err := fsl.GetDir(dir)
	if err != nil {
		return 0, 0, err
	}
	n := 0
	nbytes := np.Tlength(0)
	for _, st := range sts {
		nbytes += st.Length
		rdr, err := fsl.OpenAsyncReader(dir+"/"+st.Name, 0)
		if err != nil {
			return 0, 0, err
		}
		m := wcFile(rdr, data)
		// log.Printf("%v: %d\n", st.Name, m)
		n += m
	}
	return n, nbytes, nil
}

func Wc(fsl *fslib.FsLib, dir string, out string) (int, error) {
	data := make(Tdata)
	start := time.Now()
	n, nbytes, err := WcData(fsl, dir, data)
	wrt, err := fsl.CreateAsyncWriter(out, 0777, np.OWRITE|np.OTRUNC)
	if err != nil {
		return 0, err
	}
	defer wrt.Close()
	for k, v := range data {
		b := fmt.Sprintf("%s\t%d\n", k, v)
		_, err := wrt.Write([]byte(b))
		if err != nil {
			return 0, err
		}
	}

	ms := time.Since(start).Milliseconds()
	db.DPrintf(db.ALWAYS, "Wc %s took %vms (%s)", humanize.Bytes(uint64(nbytes)), ms, test.TputStr(nbytes, ms))
	return n, nil
}
