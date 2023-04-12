package skipinterval

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"

	"sigmaos/sessp"
)

// No overlapping intervals
func TestBasic(t *testing.T) {
	siv := MkSkipIntervals()
	e := siv.Find(*sessp.MkInterval(10, 12))
	assert.Nil(t, e)
	siv.Insert(*sessp.MkInterval(2, 4))
	siv.Insert(*sessp.MkInterval(10, 12))
	siv.Insert(*sessp.MkInterval(5, 7))
	siv.Insert(*sessp.MkInterval(0, 1))
	siv.Insert(*sessp.MkInterval(20, 22))

	e = siv.Find(*sessp.MkInterval(10, 12))
	assert.NotNil(t, e)

	e = siv.Find(*sessp.MkInterval(5, 7))
	assert.NotNil(t, e)

	siv.Delete(*sessp.MkInterval(0, 1))
	siv.Delete(*sessp.MkInterval(5, 7))
	siv.Delete(*sessp.MkInterval(10, 12))
	siv.Delete(*sessp.MkInterval(2, 4))
	siv.Delete(*sessp.MkInterval(20, 22))
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
	ivs.Delete(*sessp.MkInterval(5, 10))
	assert.Equal(t, 2, ivs.Length())
	ivs.Delete(*sessp.MkInterval(30, 50))
	assert.Equal(t, 3, ivs.Length())
	ivs.Delete(*sessp.MkInterval(50, 100))
	assert.Equal(t, 2, ivs.Length())
	ivs.Delete(*sessp.MkInterval(20, 30))
	assert.Equal(t, 2, ivs.Length())
	ivs.Delete(*sessp.MkInterval(0, 5))
	assert.Equal(t, 1, ivs.Length())
	ivs.Delete(*sessp.MkInterval(10, 20))
	assert.Equal(t, 0, ivs.Length())

	ivs.Insert(*sessp.MkInterval(0, 100))
	ivs.Delete(*sessp.MkInterval(5, 10))
	assert.Equal(t, 2, ivs.Length())
	ivs.Delete(*sessp.MkInterval(0, 100))
	assert.Equal(t, 0, ivs.Length())
}

func TestMany(t *testing.T) {
	const (
		N = 1000
		B = 10
	)
	for t := 0; t < 10; t++ {
		ivs := MkSkipIntervals()
		start := time.Now()

		for i := uint64(0); i < N; i++ {
			ivs.Insert(*sessp.MkInterval(i, i+1))
		}
		fmt.Printf("%d inserts took %v\n", N, time.Since(start))
		ivs = MkSkipIntervals()
		start = time.Now()
		for i := uint64(N * B); i > 1; i -= B {
			ivs.Insert(*sessp.MkInterval(i-1, i))
		}
		fmt.Printf("%d reverse inserts took %v\n", N, time.Since(start))
	}
}
