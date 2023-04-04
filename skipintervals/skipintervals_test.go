package skipinterval

import (
	"github.com/stretchr/testify/assert"
	"log"
	"testing"

	"sigmaos/sessp"
)

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
	siv.Insert(*sessp.MkInterval(0, 2))
	log.Printf("insert 0 siv %v\n", siv)
	e = siv.Find(*sessp.MkInterval(10, 12))
	assert.NotNil(t, e)
	log.Printf("find %v\n", e)

	siv.Delete(*sessp.MkInterval(0, 2))
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
