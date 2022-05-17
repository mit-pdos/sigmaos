package mr

import (
	// "encoding/json"
	"hash/fnv"
	"io"

	"github.com/mitchellh/mapstructure"

	np "ulambda/ninep"
)

const (
	BUFSZ = 1 << 16
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

// Result of mapper or reducer
type Result struct {
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
