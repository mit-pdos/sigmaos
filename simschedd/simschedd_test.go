package simschedd

import (
	"testing"
)

func TestRun(t *testing.T) {
	w := newWorld(1, 1)
	for i := 0; i < 10; i++ {
		w.Tick()
	}
}
