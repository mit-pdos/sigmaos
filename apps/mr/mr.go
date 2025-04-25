package mr

import (
	"fmt"
	"hash/fnv"

	"github.com/dustin/go-humanize"
	"github.com/mitchellh/mapstructure"

	"sigmaos/apps/mr/mr"
	db "sigmaos/debug"
	"sigmaos/sigmaclnt/fslib"
	sp "sigmaos/sigmap"
)

// Use Khash(key) % NReduce to choose the reduce task number for each
// KeyValue emitted by Map.
func Khash(key []byte) int {
	h := fnv.New32a()
	h.Write(key)
	return int(h.Sum32() & 0x7fffffff)
}

type Bin []mr.Split

func (b Bin) String() string {
	if len(b) == 0 {
		return fmt.Sprintf("bins (0): []")
	}
	r := fmt.Sprintf("bins (%d): [ %v, ", len(b), b[0])
	sum := sp.Tlength(b[0].Length)
	for i, s := range b[1:] {
		if s.File == b[i].File {
			r += fmt.Sprintf("{_ o %v l %v},", humanize.Bytes(uint64(s.Offset)), humanize.Bytes(uint64(s.Length)))
		} else {
			r += fmt.Sprintf("%v, ", s)
		}
		sum += s.Length
	}
	r += fmt.Sprintf("] (sum %v)", humanize.Bytes(uint64(sum)))
	return r
}

// Result of mapper or reducer
type Result struct {
	IsM      bool       `json:"IsM"`
	Task     string     `json:"Task"`
	In       sp.Tlength `json:"In"`
	Out      sp.Tlength `json:"Out"`
	OutBin   Bin        `json:"OutBin"`
	MsInner  int64      `json:"MsInner"`
	MsOuter  int64      `json:"MsOuter"`
	KernelID string     `json:"KernelID"`
}

func NewResult(data interface{}) (*Result, error) {
	r := &Result{}
	err := mapstructure.Decode(data, r)
	return r, err
}

// Each bin has a slice of splits.  Assign splits of files to a bin
// until the bin is full
func NewBins(fsl *fslib.FsLib, dir string, maxbinsz, splitsz sp.Tlength) ([]Bin, error) {
	bins := make([]Bin, 0)
	binsz := uint64(0)
	bin := Bin{}

	sts, err := fsl.GetDir(dir)
	if err != nil {
		return nil, err
	}

	currMaxBinSz := maxbinsz
	genNewBinSz := func() {
		// double := rand.Int64(10) == 0
		// if double {
		// 	currMaxBinSz = maxbinsz * 10
		// } else {
		// 	currMaxBinSz = maxbinsz
		// }
	}
	genNewBinSz()
	for x := 0; x < 5; x++ {
	for _, st := range sts {
		for i := uint64(0); ; {
			n := uint64(splitsz)
			if i+n > st.LengthUint64() {
				n = st.LengthUint64() - i
			}
			if n == 0 {
				break
			}
			split := mr.Split {
				File: dir + "/" + st.Name,
				Offset: sp.Toffset(i),
				Length: sp.Tlength(n),
			}
			bin = append(bin, split)
			binsz += n

			if binsz+uint64(splitsz) > uint64(currMaxBinSz) { // bin full?
				bins = append(bins, bin)
				bin = Bin{}
				binsz = uint64(0)
				genNewBinSz()
			}
			if n < uint64(splitsz) { // next file
				break
			}
			i += n
		}
	}
	}
	if binsz > 0 {
		bins = append(bins, bin)
	}
	db.DPrintf(db.MR, "Bin sizes: %v", bins)
	return bins, nil
}
