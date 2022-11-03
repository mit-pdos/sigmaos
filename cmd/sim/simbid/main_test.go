package main

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRun(t *testing.T) {
	ps := make(Procs, 0)
	ps = append(ps, &Proc{4, 4, 0, 0.1, 0.5})
	l, _ := ps.run(0.1)
	assert.Equal(t, l, FTick(0.5))

	//fmt.Printf("run: %v %v\n", l, ps)

	ps = append(ps, &Proc{4, 4, 0, 0.1, 0.3})
	l, _ = ps.run(0.1)
	assert.Equal(t, l, FTick(0.8))

	fmt.Printf("run: %v %v\n", l, ps)

	ps = append(ps, &Proc{4, 4, 0, 0.1, 0.4})
	l, _ = ps.run(0.1)
	assert.Equal(t, l, FTick(1.0))
	assert.Equal(t, FTick(3.5), ps[2].nTick)

	fmt.Printf("run: %v %v\n", l, ps)

	l, d := ps.run(0.1)
	assert.Equal(t, l, FTick(1.0))
	assert.Equal(t, ps[1].nTick, FTick(3.0))
	assert.Equal(t, d, Tick(0))

	fmt.Printf("run: %v %v\n", l, ps)

	ps = append(ps, &Proc{4, 4, 0, 0.1, 0.7})
	l, d = ps.run(0.1)
	assert.Equal(t, l, FTick(1.0))
	assert.Equal(t, ps[0].computeT, FTick(0.4))
	assert.Equal(t, len(ps), 2)
	assert.Equal(t, d, Tick(0))

	l, d = ps.run(0.1)

	assert.Equal(t, d, Tick(0))

	l, d = ps.run(0.1)
	l, d = ps.run(0.1)
	l, d = ps.run(0.1)
	assert.Equal(t, d, Tick(1))
	fmt.Printf("run: %v %v %v\n", l, ps, d)
}
