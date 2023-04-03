package skipinterval

import (
	"github.com/stretchr/testify/assert"
	"log"
	"testing"

	"sigmaos/sessp"
)

func TestEmpty(t *testing.T) {
	siv := MkSkipIntervals()
	log.Printf("siv %v\n", siv)
	e := siv.Find(*sessp.MkInterval(10, 2))
	assert.Nil(t, e)
	siv.Insert(*sessp.MkInterval(1, 2))
	log.Printf("siv %v\n", siv)
	siv.Insert(*sessp.MkInterval(10, 2))
	log.Printf("insert 10 siv %v\n", siv)
	siv.Insert(*sessp.MkInterval(5, 2))
	log.Printf("insert 5 siv %v\n", siv)
	siv.Insert(*sessp.MkInterval(0, 2))
	log.Printf("insert 0 siv %v\n", siv)
	e = siv.Find(*sessp.MkInterval(10, 2))
	assert.NotNil(t, e)
	log.Printf("find %v\n", e)
}
