package intervals

import (
	"log"

	"github.com/stretchr/testify/assert"
	"testing"
)

func TestInsert(t *testing.T) {
	ivs := MkIntervals()
	ivs.Insert(&interval{0, 10})
	ivs.Insert(&interval{10, 20})
	assert.Equal(t, 1, len(ivs.ivs))
	ivs.Insert(&interval{15, 20})
	assert.Equal(t, 1, len(ivs.ivs))
	ivs.Insert(&interval{30, 40})
	assert.Equal(t, 2, len(ivs.ivs))
	ivs.Insert(&interval{20, 25})
	assert.Equal(t, 2, len(ivs.ivs))
	ivs.Insert(&interval{50, 60})
	assert.Equal(t, 3, len(ivs.ivs))
	ivs.Insert(&interval{70, 80})
	assert.Equal(t, 4, len(ivs.ivs))
	ivs.Insert(&interval{40, 50})
	assert.Equal(t, 3, len(ivs.ivs))
	ivs.Insert(&interval{25, 30})
	assert.Equal(t, 2, len(ivs.ivs))
	ivs.Insert(&interval{60, 70})
	assert.Equal(t, 1, len(ivs.ivs))
}

func TestDelete(t *testing.T) {
	ivs := MkIntervals()
	ivs.Insert(&interval{0, 100})
	log.Printf("ivs %v\n", ivs.ivs)
	ivs.Delete(&interval{5, 10})
	assert.Equal(t, 2, len(ivs.ivs))
	log.Printf("ivs %v\n", ivs.ivs)
	ivs.Delete(&interval{30, 50})
	log.Printf("ivs %v\n", ivs.ivs)
	assert.Equal(t, 3, len(ivs.ivs))
	ivs.Delete(&interval{50, 100})
	log.Printf("ivs %v\n", ivs.ivs)
	assert.Equal(t, 2, len(ivs.ivs))
	ivs.Delete(&interval{20, 30})
	assert.Equal(t, 2, len(ivs.ivs))
	log.Printf("ivs %v\n", ivs.ivs)
	ivs.Delete(&interval{0, 5})
	log.Printf("ivs %v\n", ivs.ivs)
	assert.Equal(t, 1, len(ivs.ivs))
	ivs.Delete(&interval{10, 20})
	assert.Equal(t, 0, len(ivs.ivs))

	ivs.Insert(&interval{0, 100})
	log.Printf("ivs %v\n", ivs.ivs)
	ivs.Delete(&interval{5, 10})
	assert.Equal(t, 2, len(ivs.ivs))
	log.Printf("ivs %v\n", ivs.ivs)
	ivs.Delete(&interval{0, 100})
	log.Printf("ivs %v\n", ivs.ivs)
	assert.Equal(t, 0, len(ivs.ivs))
}
