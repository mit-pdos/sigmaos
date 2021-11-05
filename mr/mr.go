package mr

import (
	crand "crypto/rand"
	// "encoding/json"
	"hash/fnv"
	"log"
	"math/big"
	"os"
	"time"

	db "ulambda/debug"
	"ulambda/procinit"
)

//
// Crash testing
//

func MaybeCrash() {
	max := big.NewInt(1000)
	rr, _ := crand.Int(crand.Reader, max)
	if rr.Int64() < 330 {
		// crash!
		log.Printf("%v: Crash %v\n", db.GetName(), procinit.GetPid())
		os.Exit(1)
	} else if rr.Int64() < 660 {
		log.Printf("%v: Delay %v\n", db.GetName(), procinit.GetPid())
		// delay for a while.
		maxms := big.NewInt(10 * 1000)
		ms, _ := crand.Int(crand.Reader, maxms)
		time.Sleep(time.Duration(ms.Int64()) * time.Millisecond)
	}
}

//
// Map functions return a slice of KeyValue.
//
type KeyValue struct {
	Key   string
	Value string
}

// for sorting by key.
type ByKey []KeyValue

// for sorting by key.
func (a ByKey) Len() int           { return len(a) }
func (a ByKey) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByKey) Less(i, j int) bool { return a[i].Key < a[j].Key }

//
// use ihash(key) % NReduce to choose the reduce
// task number for each KeyValue emitted by Map.
//
func Khash(key string) int {
	h := fnv.New32a()
	h.Write([]byte(key))
	return int(h.Sum32() & 0x7fffffff)
}
