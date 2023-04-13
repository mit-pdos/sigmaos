package skipinterval

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"math/rand"
	"reflect"
	"testing"
	"time"

	// "sigmaos/intervals"
	"sigmaos/sessp"
)

// No overlapping intervals
func TestBasic(t *testing.T) {
	siv := MkSkipIntervals()
	ivs := []*sessp.Tinterval{sessp.MkInterval(2, 4), sessp.MkInterval(10, 12),
		sessp.MkInterval(5, 7), sessp.MkInterval(0, 1), sessp.MkInterval(20, 22)}
	e := siv.Find(sessp.MkInterval(10, 12))
	assert.Nil(t, e)
	for _, iv := range ivs {
		siv.Insert(iv)
	}
	for _, iv := range ivs {
		assert.True(t, siv.Present(iv))
	}
	e = siv.Find(ivs[1])
	assert.NotNil(t, e)

	siv.Delete(ivs[3])
	siv.Delete(ivs[2])
	siv.Delete(ivs[1])
	siv.Delete(ivs[0])
	siv.Delete(ivs[4])
	for _, iv := range ivs {
		assert.False(t, siv.Present(iv))
	}
	assert.True(t, siv.Length() == 0)
}

func TestInsert(t *testing.T) {
	siv := MkSkipIntervals()
	ivs := []*sessp.Tinterval{
		sessp.MkInterval(0, 10),
		sessp.MkInterval(10, 20),
		sessp.MkInterval(15, 20),
		sessp.MkInterval(30, 40),
		sessp.MkInterval(20, 25),
		sessp.MkInterval(50, 60),
		sessp.MkInterval(70, 80),
		sessp.MkInterval(40, 50),
		sessp.MkInterval(25, 30),
		sessp.MkInterval(60, 70),
	}
	lens := []int{1, 1, 1, 2, 2, 3, 4, 3, 2, 1}
	for i, iv := range ivs {
		siv.Insert(iv)
		assert.Equal(t, lens[i], siv.Length(), i)
		assert.True(t, siv.Present(iv))
	}
}

func TestDelete(t *testing.T) {
	siv := MkSkipIntervals()
	iv0 := sessp.MkInterval(0, 100)
	ivs := []*sessp.Tinterval{
		sessp.MkInterval(5, 10),
		sessp.MkInterval(30, 50),
		sessp.MkInterval(50, 100),
		sessp.MkInterval(20, 30),
		sessp.MkInterval(0, 5),
		sessp.MkInterval(10, 20),
	}
	lens := []int{2, 3, 2, 2, 1, 0}
	siv.Insert(iv0)
	for i, iv := range ivs {
		siv.Delete(iv)
		assert.Equal(t, lens[i], siv.Length(), i)
		assert.False(t, siv.Present(iv), i)
	}
	siv.Insert(iv0)
	siv.Delete(ivs[0])
	assert.True(t, siv.Present(ivs[4]))
	assert.Equal(t, 2, siv.Length())
	siv.Delete(iv0)
	assert.Equal(t, 0, siv.Length())
}

func TestRandom(t *testing.T) {
	const N = 128
	ivs := make([]*sessp.Tinterval, 0)
	siv := MkSkipIntervals()
	for i := 0; i < N; i++ {
		s := siv.rand.Int31() % N
		ivs = append(ivs, sessp.MkInterval(uint64(s), uint64(s+1)))
	}
	for _, iv := range ivs {
		siv.Insert(iv)
	}
	for _, iv := range ivs {
		assert.True(t, siv.Present(iv), iv.Marshal())
	}
}
