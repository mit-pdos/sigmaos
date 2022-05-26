package intervals

import (
	"log"

	"github.com/stretchr/testify/assert"
	"testing"

	np "ulambda/ninep"
)

func TestInsert(t *testing.T) {
	ivs := MkIntervals()
	ivs.Insert(&np.Tinterval{0, 10})
	ivs.Insert(&np.Tinterval{10, 20})
	assert.Equal(t, 1, len(ivs.ivs))
	ivs.Insert(&np.Tinterval{15, 20})
	assert.Equal(t, 1, len(ivs.ivs))
	ivs.Insert(&np.Tinterval{30, 40})
	assert.Equal(t, 2, len(ivs.ivs))
	ivs.Insert(&np.Tinterval{20, 25})
	assert.Equal(t, 2, len(ivs.ivs))
	ivs.Insert(&np.Tinterval{50, 60})
	assert.Equal(t, 3, len(ivs.ivs))
	ivs.Insert(&np.Tinterval{70, 80})
	assert.Equal(t, 4, len(ivs.ivs))
	ivs.Insert(&np.Tinterval{40, 50})
	assert.Equal(t, 3, len(ivs.ivs))
	ivs.Insert(&np.Tinterval{25, 30})
	assert.Equal(t, 2, len(ivs.ivs))
	ivs.Insert(&np.Tinterval{60, 70})
	assert.Equal(t, 1, len(ivs.ivs))
}

func TestDelete(t *testing.T) {
	ivs := MkIntervals()
	ivs.Insert(&np.Tinterval{0, 100})
	log.Printf("ivs %v\n", ivs.ivs)
	ivs.Delete(&np.Tinterval{5, 10})
	assert.Equal(t, 2, len(ivs.ivs))
	log.Printf("ivs %v\n", ivs.ivs)
	ivs.Delete(&np.Tinterval{30, 50})
	log.Printf("ivs %v\n", ivs.ivs)
	assert.Equal(t, 3, len(ivs.ivs))
	ivs.Delete(&np.Tinterval{50, 100})
	log.Printf("ivs %v\n", ivs.ivs)
	assert.Equal(t, 2, len(ivs.ivs))
	ivs.Delete(&np.Tinterval{20, 30})
	assert.Equal(t, 2, len(ivs.ivs))
	log.Printf("ivs %v\n", ivs.ivs)
	ivs.Delete(&np.Tinterval{0, 5})
	log.Printf("ivs %v\n", ivs.ivs)
	assert.Equal(t, 1, len(ivs.ivs))
	ivs.Delete(&np.Tinterval{10, 20})
	assert.Equal(t, 0, len(ivs.ivs))

	ivs.Insert(&np.Tinterval{0, 100})
	log.Printf("ivs %v\n", ivs.ivs)
	ivs.Delete(&np.Tinterval{5, 10})
	assert.Equal(t, 2, len(ivs.ivs))
	log.Printf("ivs %v\n", ivs.ivs)
	ivs.Delete(&np.Tinterval{0, 100})
	log.Printf("ivs %v\n", ivs.ivs)
	assert.Equal(t, 0, len(ivs.ivs))
}
