package simbid

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	NTRIAL                   = 1
	AVG_ARRIVAL_RATE float64 = 0.1 // per tick
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

func TestOneTenant(t *testing.T) {
}

func TestMigration(t *testing.T) {
	// policies := []Tpolicy{policyFixed, policyLast, policyBidMore}
	policies := []Tpolicy{policyBidMore}
	//policies := []Tpolicy{policyFixed}

	nNode := 50
	nTenant := 100
	nTick := Tick(1000)

	ls := make([]float64, nTenant, nTenant)
	ls[0] = 10 * AVG_ARRIVAL_RATE
	for i := 1; i < nTenant; i++ {
		ls[i] = AVG_ARRIVAL_RATE
	}

	npm := []int{1, 5, nNode}
	// npm := []int{1}

	for i := 0; i < NTRIAL; i++ {
		for _, p := range policies {
			for _, n := range npm {
				runSim(mkWorld(nNode, nTenant, n, ls, nTick, p))
			}
		}
	}
}
