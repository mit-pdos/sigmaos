package intervals_test

import (
	"github.com/stretchr/testify/assert"
	"testing"

	db "sigmaos/debug"
	"sigmaos/intervals"
	"sigmaos/sessp"
)

func TestSimple(t *testing.T) {
	ivs := intervals.MkIntervals()
	ivs.Insert(sessp.MkInterval(1, 2))
	ivs.Insert(sessp.MkInterval(2, 3))
	ivs.Delete(sessp.MkInterval(1, 2))
	assert.Equal(t, 1, ivs.Size())
}

func TestContains(t *testing.T) {
	ivs := intervals.MkIntervals()
	ivs.Insert(sessp.MkInterval(0, 10))
	ivs.Insert(sessp.MkInterval(90, 100))
	assert.True(t, ivs.Contains(0))
	assert.False(t, ivs.Contains(10))
	assert.False(t, ivs.Contains(11))
	assert.True(t, ivs.Contains(90))
	assert.False(t, ivs.Contains(100))
	assert.False(t, ivs.Contains(200))
}

func TestInsert(t *testing.T) {
	ivs := intervals.MkIntervals()
	ivs.Insert(sessp.MkInterval(0, 10))
	ivs.Insert(sessp.MkInterval(10, 20))
	assert.Equal(t, 1, ivs.Size())
	ivs.Insert(sessp.MkInterval(15, 20))
	assert.Equal(t, 1, ivs.Size())
	ivs.Insert(sessp.MkInterval(30, 40))
	assert.Equal(t, 2, ivs.Size())
	ivs.Insert(sessp.MkInterval(20, 25))
	assert.Equal(t, 2, ivs.Size())
	ivs.Insert(sessp.MkInterval(50, 60))
	assert.Equal(t, 3, ivs.Size())
	ivs.Insert(sessp.MkInterval(70, 80))
	assert.Equal(t, 4, ivs.Size())
	ivs.Insert(sessp.MkInterval(40, 50))
	assert.Equal(t, 3, ivs.Size())
	ivs.Insert(sessp.MkInterval(25, 30))
	assert.Equal(t, 2, ivs.Size())
	ivs.Insert(sessp.MkInterval(60, 70))
	assert.Equal(t, 1, ivs.Size())
}

func TestDelete(t *testing.T) {
	ivs := intervals.MkIntervals()
	ivs.Insert(sessp.MkInterval(0, 100))
	db.DPrintf(db.TEST, "ivs %v\n", ivs)
	ivs.Delete(sessp.MkInterval(5, 10))
	assert.Equal(t, 2, ivs.Size())
	db.DPrintf(db.TEST, "ivs %v\n", ivs)
	ivs.Delete(sessp.MkInterval(30, 50))
	db.DPrintf(db.TEST, "ivs %v\n", ivs)
	assert.Equal(t, 3, ivs.Size())
	ivs.Delete(sessp.MkInterval(50, 100))
	db.DPrintf(db.TEST, "ivs %v\n", ivs)
	assert.Equal(t, 2, ivs.Size())
	ivs.Delete(sessp.MkInterval(20, 30))
	assert.Equal(t, 2, ivs.Size())
	db.DPrintf(db.TEST, "ivs %v\n", ivs)
	ivs.Delete(sessp.MkInterval(0, 5))
	db.DPrintf(db.TEST, "ivs %v\n", ivs)
	assert.Equal(t, 1, ivs.Size())
	ivs.Delete(sessp.MkInterval(10, 20))
	assert.Equal(t, 0, ivs.Size())

	ivs.Insert(sessp.MkInterval(0, 100))
	db.DPrintf(db.TEST, "ivs %v\n", ivs)
	ivs.Delete(sessp.MkInterval(5, 10))
	assert.Equal(t, 2, ivs.Size())
	db.DPrintf(db.TEST, "ivs %v\n", ivs)
	ivs.Delete(sessp.MkInterval(0, 100))
	db.DPrintf(db.TEST, "ivs %v\n", ivs)
	assert.Equal(t, 0, ivs.Size())
}

func processIV(t *testing.T, retrieved map[uint64]bool, iv *sessp.Tinterval) {
	for i := iv.Start; i < iv.End; i++ {
		assert.False(t, retrieved[i], "Double retrieve %v", i)
		retrieved[i] = true
	}
}

// Check that the retrieved map contains all integers in the range
// [start, end], and not any integers before this range, which have not already
// been checked.
func checkRetrieved(t *testing.T, retrieved map[uint64]bool, prevend, start, end uint64) {
	if prevend > 0 {
		for i := prevend; i < start; i++ {
			assert.False(t, retrieved[i], "Contains %v", i)
		}
	}
	for i := start; i <= end; i++ {
		assert.True(t, retrieved[i], "Doesn't contain %v", i)
	}
}

func doNext(t *testing.T, ivs *intervals.Intervals, retrieved map[uint64]bool, starts, ends []uint64) {
	var iv *sessp.Tinterval
	assert.Equal(t, len(starts), len(ends))
	iv = ivs.Next()
	if !assert.NotNil(t, iv) {
		db.DFatalf("Error")
	}
	processIV(t, retrieved, iv)
	for i := range starts {
		start := starts[i]
		end := ends[i]
		var prevend uint64
		if i > 0 {
			prevend = ends[i-1] + 1
		}
		checkRetrieved(t, retrieved, prevend, start, end)
	}
}

// Spec we are testing for:
// * Unless ivs.ResetNext is called, the same number should never be returned
// twice from ivs.Next, assuming it was never inserted twice.
// * All intervals inserted in ivs will eventually be returned by Next.
func TestNextInsert(t *testing.T) {
	retrieved := make(map[uint64]bool)
	ivs := intervals.MkIntervals()
	ivs.Insert(sessp.MkInterval(0, 10))
	doNext(t, ivs, retrieved, []uint64{0}, []uint64{9})
	assert.Equal(t, 1, ivs.Size())
	ivs.Insert(sessp.MkInterval(10, 20))
	doNext(t, ivs, retrieved, []uint64{0}, []uint64{19})
	assert.Equal(t, 1, ivs.Size())
	ivs.Insert(sessp.MkInterval(25, 26))
	ivs.Insert(sessp.MkInterval(22, 23))
	ivs.Insert(sessp.MkInterval(23, 24))
	assert.Equal(t, 3, ivs.Size())
	doNext(t, ivs, retrieved, []uint64{0, 22}, []uint64{19, 23})
	ivs.Insert(sessp.MkInterval(21, 22))
	assert.Equal(t, 3, ivs.Size())
	doNext(t, ivs, retrieved, []uint64{0, 21}, []uint64{19, 23})
	ivs.Insert(sessp.MkInterval(25, 101))
	assert.Equal(t, 3, ivs.Size())
	doNext(t, ivs, retrieved, []uint64{0, 21, 25}, []uint64{19, 23, 100})
	ivs.Insert(sessp.MkInterval(20, 21))
	assert.Equal(t, 2, ivs.Size())
	doNext(t, ivs, retrieved, []uint64{0, 20, 25}, []uint64{19, 23, 100})
	ivs.Insert(sessp.MkInterval(24, 25))
	assert.Equal(t, 1, ivs.Size())
	doNext(t, ivs, retrieved, []uint64{0}, []uint64{100})
}

func TestNextDelete(t *testing.T) {
	retrieved := make(map[uint64]bool)
	ivs := intervals.MkIntervals()
	ivs.Insert(sessp.MkInterval(0, 10))
	doNext(t, ivs, retrieved, []uint64{0}, []uint64{9})
	assert.Equal(t, 1, ivs.Size())
	ivs.Insert(sessp.MkInterval(10, 20))
	doNext(t, ivs, retrieved, []uint64{0}, []uint64{19})
	assert.Equal(t, 1, ivs.Size())
	ivs.Insert(sessp.MkInterval(25, 26))
	ivs.Insert(sessp.MkInterval(22, 23))
	ivs.Insert(sessp.MkInterval(23, 24))
	assert.Equal(t, 3, ivs.Size())
	doNext(t, ivs, retrieved, []uint64{0, 22}, []uint64{19, 23})
	ivs.Insert(sessp.MkInterval(21, 22))
	assert.Equal(t, 3, ivs.Size())
	doNext(t, ivs, retrieved, []uint64{0, 21}, []uint64{19, 23})
	ivs.Insert(sessp.MkInterval(25, 101))
	assert.Equal(t, 3, ivs.Size())
	ivs.Delete(sessp.MkInterval(50, 51))
	assert.Equal(t, 4, ivs.Size())
	doNext(t, ivs, retrieved, []uint64{0, 21, 25}, []uint64{19, 23, 49})
	doNext(t, ivs, retrieved, []uint64{0, 21, 25, 51}, []uint64{19, 23, 49, 100})
	ivs.Insert(sessp.MkInterval(20, 21))
	doNext(t, ivs, retrieved, []uint64{0, 20, 25, 51}, []uint64{19, 23, 49, 100})
	ivs.Insert(sessp.MkInterval(24, 25))
	doNext(t, ivs, retrieved, []uint64{0, 51}, []uint64{49, 100})
	ivs.Insert(sessp.MkInterval(50, 51))
	doNext(t, ivs, retrieved, []uint64{0}, []uint64{100})
}

func TestNextReset(t *testing.T) {
	retrieved := make(map[uint64]bool)
	ivs := intervals.MkIntervals()
	ivs.Insert(sessp.MkInterval(0, 10))
	doNext(t, ivs, retrieved, []uint64{0}, []uint64{9})
	assert.Equal(t, 1, ivs.Size())
	ivs.Insert(sessp.MkInterval(10, 20))
	doNext(t, ivs, retrieved, []uint64{0}, []uint64{19})
	assert.Equal(t, 1, ivs.Size())
	ivs.Insert(sessp.MkInterval(25, 26))
	ivs.Insert(sessp.MkInterval(22, 23))
	ivs.Insert(sessp.MkInterval(23, 24))
	assert.Equal(t, 3, ivs.Size())
	doNext(t, ivs, retrieved, []uint64{0, 22}, []uint64{19, 23})
	ivs.Insert(sessp.MkInterval(21, 22))
	assert.Equal(t, 3, ivs.Size())
	doNext(t, ivs, retrieved, []uint64{0, 21}, []uint64{19, 23})
	ivs.Insert(sessp.MkInterval(25, 101))
	assert.Equal(t, 3, ivs.Size())
	ivs.Delete(sessp.MkInterval(50, 51))
	assert.Equal(t, 4, ivs.Size())
	doNext(t, ivs, retrieved, []uint64{0, 21, 25}, []uint64{19, 23, 49})
	doNext(t, ivs, retrieved, []uint64{0, 21, 25, 51}, []uint64{19, 23, 49, 100})
	ivs.Insert(sessp.MkInterval(20, 21))
	doNext(t, ivs, retrieved, []uint64{0, 20, 25, 51}, []uint64{19, 23, 49, 100})
	newRetrieved := make(map[uint64]bool)
	ivs.ResetNext()
	doNext(t, ivs, newRetrieved, []uint64{0}, []uint64{19})
	ivs.Insert(sessp.MkInterval(24, 25))
	ivs.Insert(sessp.MkInterval(50, 51))
	doNext(t, ivs, newRetrieved, []uint64{0}, []uint64{100})
}
