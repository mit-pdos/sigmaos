package memblock

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Which machines get dedicated has to be a property of the cluster, not of the
// order a directory listing came back in, or two runs of the same experiment
// aren't comparable.
func TestFirstNIsDeterministic(t *testing.T) {
	kids := []string{"sigma-c", "sigma-a", "sigma-d", "sigma-b"}
	got, err := firstN(kids, 2)
	assert.Nil(t, err)
	assert.Equal(t, []string{"sigma-a", "sigma-b"}, got)

	// Same set, different listing order, same answer.
	got2, err := firstN([]string{"sigma-d", "sigma-b", "sigma-c", "sigma-a"}, 2)
	assert.Nil(t, err)
	assert.Equal(t, got, got2)
}

func TestFirstNAll(t *testing.T) {
	kids := []string{"b", "a", "c"}
	for _, n := range []int{0, -1} {
		got, err := firstN(kids, n)
		assert.Nil(t, err)
		assert.Equal(t, []string{"a", "b", "c"}, got, "n %v should mean all of them", n)
	}
}

// Asking for more machines than the cluster has is an error, not "take what there
// is": an experiment sized around n dedicated machines is wrong, not smaller, if
// it silently gets fewer.
func TestFirstNTooMany(t *testing.T) {
	_, err := firstN([]string{"a", "b"}, 3)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "cluster has 2")
}
