package mr

import (
	// "encoding/json"
	"hash/fnv"
	"io"
	"io/fs"

	"github.com/mitchellh/mapstructure"

	np "ulambda/ninep"
)

const (
	BINSZ   np.Tlength = 1 << 17
	SPLITSZ np.Tlength = BINSZ >> 2
	BUFSZ              = 1 << 16
)

//
// Map and reduce functions produce KeyValue pairs
//

type KeyValue struct {
	K string
	V string
}

type EmitT func(*KeyValue) error

type ReduceT func(string, []string, EmitT) error
type MapT func(string, io.Reader, EmitT) error

// for sorting by key.
type ByKey []*KeyValue

// for sorting by key.
func (a ByKey) Len() int           { return len(a) }
func (a ByKey) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByKey) Less(i, j int) bool { return a[i].K < a[j].K }

//
// use ihash(key) % NReduce to choose the reduce
// task number for each KeyValue emitted by Map.
//
func Khash(key string) int {
	h := fnv.New32a()
	h.Write([]byte(key))
	return int(h.Sum32() & 0x7fffffff)
}

// An input split
type Split struct {
	File   string     `json:"File"`
	Offset np.Toffset `json:"Offset"`
	Length np.Tlength `json:"Length"`
}

type Bin []Split

// Result of mapper or reducer
type Result struct {
	IsM  bool       `json:"IsM"`
	Task string     `json:"Task"`
	In   np.Tlength `json:"In"`
	Out  np.Tlength `json:"Out"`
	Ms   int64      `json:"Ms"`
}

func mkResult(data interface{}) *Result {
	r := &Result{}
	mapstructure.Decode(data, r)
	return r
}

// Each bin has a slice of splits.  Assign splits of files to a bin
// until the bin is file.
func MkBins(files []fs.FileInfo) []Bin {
	bins := make([]Bin, 0)
	binsz := np.Tlength(0)
	bin := Bin{}
	for _, f := range files {
		for i := np.Tlength(0); ; {
			n := SPLITSZ
			if i+n > np.Tlength(f.Size()) {
				n = np.Tlength(f.Size()) - i
			}
			split := Split{MIN + f.Name(), np.Toffset(i), n}
			bin = append(bin, split)
			binsz += n
			if binsz+SPLITSZ > BINSZ { // bin full?
				bins = append(bins, bin)
				bin = Bin{}
				binsz = np.Tlength(0)
			}
			if n < SPLITSZ { // next file
				break
			}
			i += n
		}
	}
	bins = append(bins, bin)
	return bins
}
