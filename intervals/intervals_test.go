package intervals

import (
	"log"

	"github.com/stretchr/testify/assert"
	"testing"
)

func TestInsert(t *testing.T) {
	ivs := MkIntervals()
	ivs.Insert(&Interval{0, 10})
	ivs.Insert(&Interval{10, 20})
	assert.Equal(t, 1, len(ivs.ivs))
	ivs.Insert(&Interval{15, 20})
	assert.Equal(t, 1, len(ivs.ivs))
	ivs.Insert(&Interval{30, 40})
	assert.Equal(t, 2, len(ivs.ivs))
	ivs.Insert(&Interval{20, 25})
	assert.Equal(t, 2, len(ivs.ivs))
	ivs.Insert(&Interval{50, 60})
	assert.Equal(t, 3, len(ivs.ivs))
	ivs.Insert(&Interval{70, 80})
	assert.Equal(t, 4, len(ivs.ivs))
	ivs.Insert(&Interval{40, 50})
	assert.Equal(t, 3, len(ivs.ivs))
	ivs.Insert(&Interval{25, 30})
	assert.Equal(t, 2, len(ivs.ivs))
	ivs.Insert(&Interval{60, 70})
	assert.Equal(t, 1, len(ivs.ivs))
}

func TestDelete(t *testing.T) {
	ivs := MkIntervals()
	ivs.Insert(&Interval{0, 100})
	log.Printf("ivs %v\n", ivs.ivs)
	ivs.Delete(&Interval{5, 10})
	assert.Equal(t, 2, len(ivs.ivs))
	log.Printf("ivs %v\n", ivs.ivs)
	ivs.Delete(&Interval{30, 50})
	log.Printf("ivs %v\n", ivs.ivs)
	assert.Equal(t, 3, len(ivs.ivs))
	ivs.Delete(&Interval{50, 100})
	log.Printf("ivs %v\n", ivs.ivs)
	assert.Equal(t, 2, len(ivs.ivs))
	ivs.Delete(&Interval{20, 30})
	assert.Equal(t, 2, len(ivs.ivs))
	log.Printf("ivs %v\n", ivs.ivs)
	ivs.Delete(&Interval{0, 5})
	log.Printf("ivs %v\n", ivs.ivs)
	assert.Equal(t, 1, len(ivs.ivs))
	ivs.Delete(&Interval{10, 20})
	assert.Equal(t, 0, len(ivs.ivs))

	ivs.Insert(&Interval{0, 100})
	log.Printf("ivs %v\n", ivs.ivs)
	ivs.Delete(&Interval{5, 10})
	assert.Equal(t, 2, len(ivs.ivs))
	log.Printf("ivs %v\n", ivs.ivs)
	ivs.Delete(&Interval{0, 100})
	log.Printf("ivs %v\n", ivs.ivs)
	assert.Equal(t, 0, len(ivs.ivs))
}
