package main

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRun(t *testing.T) {
	ps := make(Procs, 0)
	ps = append(ps, &Proc{4, 4, 0.1, 0.5})
	l := ps.run(0.1)
	assert.Equal(t, l, Load(0.5))

	ps = append(ps, &Proc{4, 4, 0.1, 0.3})
	l = ps.run(0.1)
	assert.Equal(t, l, Load(0.8))

	ps = append(ps, &Proc{4, 4, 0.1, 0.4})
	l = ps.run(0.1)
	assert.Equal(t, l, Load(1.0))
	assert.Equal(t, ps[2].nTick, FTick(3.5))

	l = ps.run(0.1)
	fmt.Printf("run: %v %v\n", l, ps)
	assert.Equal(t, l, Load(1.0))
	assert.Equal(t, ps[2].nTick, FTick(3.0))
}
