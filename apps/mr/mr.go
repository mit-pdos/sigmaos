package mr

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"io"

	"github.com/dustin/go-humanize"

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

// RESTART is the message a reducer returns when it could not read a mapper's
// output and the mapper must be re-run. Produced by the reducer, interpreted by
// the coordinator.
const RESTART = "restart"

// TreduceTask is a reducer's task data: which reducer it is, and the mapper
// output shards it reads. Written by the coordinator, read by the reducer.
type TreduceTask struct {
	Task  string `json:"Task"`
	Input Bin
}

type Bin []mr.Split

// Threshold (the sigmap max message size) above which a bin's JSON
// representation is compressed when marshaled. Compression is only needed
// for very large bins (e.g., a reduce task's input bin, which contains one
// split per mapper task, so with tens of thousands of mappers its JSON
// representation grows to several MiB), which would otherwise exceed the
// sigmap max message size (and etcd's max request size, 1.5MiB by default)
// when passed around via fttask RPCs. Smaller bins are left as plain
// (human-readable) JSON.
const COMPRESS_BINSZ = int(sp.MAXGETSET)

// Marshal a bin as a plain JSON array of splits if it is small, and as
// gzip-compressed JSON (base64-encoded, since JSON cannot hold raw bytes) if
// it is large. Compression is effective because split JSON is highly
// repetitive: splits in a bin share most of their file path.
func (b Bin) MarshalJSON() ([]byte, error) {
	d, err := json.Marshal([]mr.Split(b))
	if err != nil {
		return nil, err
	}
	if len(d) <= COMPRESS_BINSZ {
		return d, nil
	}
	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	if _, err := w.Write(d); err != nil {
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	return json.Marshal(buf.Bytes())
}

func (b *Bin) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, []byte("null")) {
		*b = nil
		return nil
	}
	// Plain (uncompressed) representation
	if len(data) > 0 && data[0] == '[' {
		var splits []mr.Split
		if err := json.Unmarshal(data, &splits); err != nil {
			return err
		}
		*b = Bin(splits)
		return nil
	}
	var zd []byte
	if err := json.Unmarshal(data, &zd); err != nil {
		return err
	}
	r, err := gzip.NewReader(bytes.NewReader(zd))
	if err != nil {
		return err
	}
	defer r.Close()
	d, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	var splits []mr.Split
	if err := json.Unmarshal(d, &splits); err != nil {
		return err
	}
	*b = Bin(splits)
	return nil
}

func (b Bin) String() string {
	if len(b) == 0 {
		return "bins (0): []"
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

// Wall-clock duration of a job's map and reduce phases, as measured by the
// coordinator and persisted for the driver to report.
type PhaseDurations struct {
	MapMs    int64 `json:"MapMs"`
	ReduceMs int64 `json:"ReduceMs"`
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

// Decode a Result from generically-unmarshaled JSON (e.g., a proc exit
// status' data). Round-trip through JSON (rather than using something like
// mapstructure) so that Bin's custom JSON encoding is applied when decoding
// OutBin.
func NewResult(data interface{}) (*Result, error) {
	r := &Result{}
	b, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(b, r); err != nil {
		return nil, err
	}
	return r, nil
}

// Each bin has a slice of splits.  Assign splits of files to a bin
// until the bin is full
func NewBins(fsl *fslib.FsLib, inputDir string, swapLocalForAny bool, maxbinsz, splitsz sp.Tlength) ([]Bin, error) {
	bins := make([]Bin, 0)
	binsz := uint64(0)
	bin := Bin{}

	dir := inputDir
	if swapLocalForAny {
		dir, _ = sp.SubstLocal(dir, sp.ANY)
	}
	sts, err := fsl.GetDir(dir)
	if err != nil {
		return nil, err
	}

	for _, st := range sts {
		for i := uint64(0); ; {
			n := uint64(splitsz)
			if i+n > st.LengthUint64() {
				n = st.LengthUint64() - i
			}
			if n == 0 {
				break
			}
			split := mr.Split{
				File:   inputDir + "/" + st.Name,
				Offset: sp.Toffset(i),
				Length: sp.Tlength(n),
			}
			bin = append(bin, split)
			binsz += n

			if binsz+uint64(splitsz) >= uint64(maxbinsz) { // bin full?
				bins = append(bins, bin)
				bin = Bin{}
				binsz = uint64(0)
			}
			if n < uint64(splitsz) { // next file
				break
			}
			i += n
		}
	}
	if binsz > 0 {
		bins = append(bins, bin)
	}
	db.DPrintf(db.MR, "Bin sizes: %v", bins)
	return bins, nil
}
