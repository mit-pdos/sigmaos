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
	db.DPrintf(db.ALWAYS, "ivs %v\n", ivs)
	ivs.Delete(sessp.MkInterval(5, 10))
	assert.Equal(t, 2, ivs.Size())
	db.DPrintf(db.ALWAYS, "ivs %v\n", ivs)
	ivs.Delete(sessp.MkInterval(30, 50))
	db.DPrintf(db.ALWAYS, "ivs %v\n", ivs)
	assert.Equal(t, 3, ivs.Size())
	ivs.Delete(sessp.MkInterval(50, 100))
	db.DPrintf(db.ALWAYS, "ivs %v\n", ivs)
	assert.Equal(t, 2, ivs.Size())
	ivs.Delete(sessp.MkInterval(20, 30))
	assert.Equal(t, 2, ivs.Size())
	db.DPrintf(db.ALWAYS, "ivs %v\n", ivs)
	ivs.Delete(sessp.MkInterval(0, 5))
	db.DPrintf(db.ALWAYS, "ivs %v\n", ivs)
	assert.Equal(t, 1, ivs.Size())
	ivs.Delete(sessp.MkInterval(10, 20))
	assert.Equal(t, 0, ivs.Size())

	ivs.Insert(sessp.MkInterval(0, 100))
	db.DPrintf(db.ALWAYS, "ivs %v\n", ivs)
	ivs.Delete(sessp.MkInterval(5, 10))
	assert.Equal(t, 2, ivs.Size())
	db.DPrintf(db.ALWAYS, "ivs %v\n", ivs)
	ivs.Delete(sessp.MkInterval(0, 100))
	db.DPrintf(db.ALWAYS, "ivs %v\n", ivs)
	assert.Equal(t, 0, ivs.Size())
}
