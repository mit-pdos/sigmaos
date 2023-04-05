package skipinterval

import (
	"github.com/stretchr/testify/assert"
	"log"
	"testing"

	db "sigmaos/debug"
	"sigmaos/sessp"
)

// No overlapping intervals
func TestBasic(t *testing.T) {
	siv := MkSkipIntervals()
	log.Printf("siv %v\n", siv)
	e := siv.Find(*sessp.MkInterval(10, 12))
	assert.Nil(t, e)
	siv.Insert(*sessp.MkInterval(2, 4))
	log.Printf("siv %v\n", siv)
	siv.Insert(*sessp.MkInterval(10, 12))
	log.Printf("insert 10 siv %v\n", siv)
	siv.Insert(*sessp.MkInterval(5, 7))
	log.Printf("insert 5 siv %v\n", siv)
	siv.Insert(*sessp.MkInterval(0, 1))
	log.Printf("insert 0 siv %v\n", siv)
	e = siv.Find(*sessp.MkInterval(10, 12))
	assert.NotNil(t, e)
	log.Printf("find %v\n", e)

	siv.Delete(*sessp.MkInterval(0, 1))
	log.Printf("del 0 siv %v\n", siv)
	siv.Delete(*sessp.MkInterval(5, 7))
	log.Printf("del 5 siv %v\n", siv)
	siv.Delete(*sessp.MkInterval(10, 12))
	log.Printf("del 5 siv %v\n", siv)
	siv.Delete(*sessp.MkInterval(2, 4))
	log.Printf("del 5 siv %v\n", siv)
}

func TestInsert(t *testing.T) {
	ivs := MkSkipIntervals()
	ivs.Insert(*sessp.MkInterval(0, 10))
	ivs.Insert(*sessp.MkInterval(10, 20))
	assert.Equal(t, 1, ivs.Length())
	ivs.Insert(*sessp.MkInterval(15, 20))
	assert.Equal(t, 1, ivs.Length())
	ivs.Insert(*sessp.MkInterval(30, 40))
	assert.Equal(t, 2, ivs.Length())
	ivs.Insert(*sessp.MkInterval(20, 25))
	assert.Equal(t, 2, ivs.Length())
	ivs.Insert(*sessp.MkInterval(50, 60))
	assert.Equal(t, 3, ivs.Length())
	ivs.Insert(*sessp.MkInterval(70, 80))
	assert.Equal(t, 4, ivs.Length())
	ivs.Insert(*sessp.MkInterval(40, 50))
	assert.Equal(t, 3, ivs.Length())
	ivs.Insert(*sessp.MkInterval(25, 30))
	assert.Equal(t, 2, ivs.Length())
	ivs.Insert(*sessp.MkInterval(60, 70))
	assert.Equal(t, 1, ivs.Length())
}

func TestDelete(t *testing.T) {
	ivs := MkSkipIntervals()
	ivs.Insert(*sessp.MkInterval(0, 100))
	db.DPrintf(db.ALWAYS, "ivs %v\n", ivs)
	ivs.Delete(*sessp.MkInterval(5, 10))
	assert.Equal(t, 2, ivs.Length())
	db.DPrintf(db.ALWAYS, "ivs %v\n", ivs)
	ivs.Delete(*sessp.MkInterval(30, 50))
	db.DPrintf(db.ALWAYS, "ivs %v\n", ivs)
	assert.Equal(t, 3, ivs.Length())
	ivs.Delete(*sessp.MkInterval(50, 100))
	db.DPrintf(db.ALWAYS, "ivs %v\n", ivs)
	assert.Equal(t, 2, ivs.Length())
	ivs.Delete(*sessp.MkInterval(20, 30))
	assert.Equal(t, 2, ivs.Length())
	db.DPrintf(db.ALWAYS, "ivs %v\n", ivs)
	ivs.Delete(*sessp.MkInterval(0, 5))
	db.DPrintf(db.ALWAYS, "ivs %v\n", ivs)
	assert.Equal(t, 1, ivs.Length())
	ivs.Delete(*sessp.MkInterval(10, 20))
	assert.Equal(t, 0, ivs.Length())

	ivs.Insert(*sessp.MkInterval(0, 100))
	db.DPrintf(db.ALWAYS, "ivs %v\n", ivs)
	ivs.Delete(*sessp.MkInterval(5, 10))
	assert.Equal(t, 2, ivs.Length())
	db.DPrintf(db.ALWAYS, "ivs %v\n", ivs)
	ivs.Delete(*sessp.MkInterval(0, 100))
	db.DPrintf(db.ALWAYS, "ivs %v\n", ivs)
	assert.Equal(t, 0, ivs.Length())
}

func TestMany(t *testing.T) {
	const (
		N = 100
		B = 10
	)

	ivs := MkSkipIntervals()
	for i := uint64(0); i < N; i++ {
		ivs.Insert(*sessp.MkInterval(i, i+1))
	}
	log.Printf("skipl %v\n", ivs)
	//for i := uint64(0); i < N/B; i += B {
	//	ivs.Delete(*sessp.MkInterval(i, i+B))
	//}
}
