package simbid

import (
	"fmt"
	"math"
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

	fmt.Printf("run: %v %v\n", l, ps)

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

	//fmt.Printf("run: %v %v\n", l, ps)

	ps = append(ps, &Proc{4, 4, 0, 0.1, 0.7})
	l, d = ps.run(0.1)
	assert.Equal(t, l, FTick(1.0))
	assert.Equal(t, ps[0].ci, FTick(0.4))
	assert.Equal(t, len(ps), 2)
	assert.Equal(t, d, Tick(0))

	l, d = ps.run(0.1)

	assert.Equal(t, d, Tick(0))

	l, d = ps.run(0.1)
	l, d = ps.run(0.1)
	l, d = ps.run(0.1)
	assert.Equal(t, d, Tick(1))
	//fmt.Printf("run: %v %v %v\n", l, ps, d)
}

const HIGH = 10

func newArrival(t int) []float64 {
	ls := make([]float64, t, t)
	ls[0] = HIGH * AVG_ARRIVAL_RATE
	for i := 1; i < t; i++ {
		ls[i] = AVG_ARRIVAL_RATE
	}
	return ls
}

func newArrivalExp(t int) ([]float64, float64) {
	ls := make([]float64, t, t)
	s := 0
	n := t / 2
	h := float64(1)
	sum := float64(0)
	for {
		// fmt.Printf("s = %d %d %f\n", s, n, h)
		for i := s; i < s+n; i++ {
			ls[i] = h * AVG_ARRIVAL_RATE
			sum += ls[i]
		}
		s = s + n
		h = h * 2
		n = n / 2
		if n == 0 {
			ls[s] = h * AVG_ARRIVAL_RATE
			break
		}
	}
	return ls, sum
}

func TestOneTenant(t *testing.T) {
	nTenant := 1
	nTick := 100
	nNode := 50
	ls := newArrival(nTenant)
	w := newWorld(nNode, nTenant, 1, ls, Tick(nTick), policyFixed, 1.0)
	sim := runSim(w)
	ten := &sim.tenants[0]
	// sim.stats()
	assert.True(t, sim.nproc > nTick-10 && sim.nproc < nTick+10)
	assert.True(t, ten.maxnode <= MAX_SERVICE_TIME+1)
	assert.True(t, int(sim.proclen)/sim.nproc == (MAX_SERVICE_TIME+1)/2)
	assert.True(t, int(ten.ntick)/ten.nproc == (MAX_SERVICE_TIME+1)/2)
	assert.True(t, int(ten.nwork)/ten.nproc == (MAX_SERVICE_TIME+1)/2)
	assert.True(t, ten.nidle == 0)
	assert.True(t, ten.nwait == 0)
	assert.True(t, ten.ndelay == 0)
	assert.True(t, ten.nevict == 0)
	assert.True(t, ten.nmigrate == 0)
	assert.True(t, float64(sim.nprocq)/float64(sim.world.nTick) == 0)
	assert.True(t, sim.maxqNode == 1)
}

func TestWait(t *testing.T) {
	nTenant := 1
	nTick := 100
	nNode := 5
	ls := newArrival(nTenant)
	w := newWorld(nNode, nTenant, 1, ls, Tick(nTick), policyFixed, 1.0)
	sim := runSim(w)
	//sim.stats()
	r := float64(sim.nprocq) / float64(sim.world.nTick)
	// fmt.Printf("%f\n", r)
	assert.True(t, r >= MAX_SERVICE_TIME)
	assert.True(t, r < MAX_SERVICE_TIME+1)
}

func TestComputeI(t *testing.T) {
	nTenant := 1
	nTick := 100
	nNode := 50
	ls := newArrival(nTenant)
	w := newWorld(nNode, nTenant, 1, ls, Tick(nTick), policyFixed, 0.5)
	sim := runSim(w)
	// sim.stats()
	ten := &sim.tenants[0]
	assert.True(t, sim.nproc > nTick-10 && sim.nproc < nTick+10)
	assert.True(t, ten.maxnode >= HIGH/2)
	assert.True(t, int(math.Round(float64(sim.proclen)/float64(sim.nproc))) == (MAX_SERVICE_TIME+1)/2)
	assert.True(t, int(ten.ntick)/ten.nproc >= (MAX_SERVICE_TIME+1)/4)
	assert.True(t, int(ten.ntick)/ten.nproc < (MAX_SERVICE_TIME+1)/2)
	assert.True(t, int(ten.nwork)/ten.nproc == (MAX_SERVICE_TIME+1)/4)
	assert.True(t, ten.nidle > 0)
}

func TestFixedVsLast(t *testing.T) {
	nNode := 35
	nTenant := 100
	nTick := Tick(1000)
	ls := newArrival(nTenant)
	sims := make([]*Sim, 0)
	policies := []Tpolicy{policyFixed, policyLast}
	for _, p := range policies {
		w := newWorld(nNode, nTenant, 1, ls, nTick, p, 0.5)
		s := runSim(w)
		sims = append(sims, s)
	}
	n := float64(nTick)
	assert.True(t, float64(sims[0].nprocq)/n > 10*float64(sims[1].nprocq)/n)
}

func TestReserveNode(t *testing.T) {
	nNode := 35
	nTenant := 100
	nTick := Tick(1000)
	ls := newArrival(nTenant)
	sims := make([]*Sim, 0)
	policies := []Tpolicy{policyLast, policyBidMore}
	cis := []FTick{0.5, 1.0}
	for _, ci := range cis {
		for _, p := range policies {
			w := newWorld(nNode, nTenant, 1, ls, nTick, p, ci)
			s := runSim(w)
			//s.stats()
			sims = append(sims, s)
		}
	}
	r0 := float64(sims[0].tenants[0].nevict) / float64(sims[1].tenants[0].nevict)
	r1 := float64(sims[2].tenants[0].nevict) / float64(sims[3].tenants[0].nevict)
	fmt.Printf("r0 %f r1 %f\n", r0, r1)
	assert.True(t, r0 >= 1.5)
	assert.True(t, r1 >= 1.2)
}

func TestMigration(t *testing.T) {
	nTenant := 100
	nTick := Tick(1000)
	ls := newArrival(nTenant)
	npms := []int{1, 5}
	nnodes := []int{35, 50}
	sims := make([]*Sim, 0)
	for _, n := range nnodes {
		for _, npm := range npms {
			w := newWorld(n, nTenant, npm, ls, nTick, policyBidMore, 0.5)
			s := runSim(w)
			//s.stats()
			sims = append(sims, s)
		}
	}
	r0 := float64(sims[0].tenants[0].nevict) / float64(sims[1].tenants[0].nevict)
	r1 := float64(sims[2].tenants[0].nevict) / float64(sims[3].tenants[0].nevict)

	r00 := float64(sims[0].mgr.nevict) / float64(sims[1].mgr.nevict)
	r11 := float64(sims[2].mgr.nevict) / float64(sims[3].mgr.nevict)

	fmt.Printf("%f %f %f %f\n", r0, r1, r00, r11)
	assert.True(t, r0 > 1.3)
	assert.True(t, r1 > 1.0)
}

func TestArrivalExp(t *testing.T) {
	nTenant := 128
	nTick := Tick(1000)
	ls, sum := newArrivalExp(nTenant)
	fmt.Printf("sum %f\n", sum)
	npms := []int{1, 5}
	nnodes := []int{200, 225}
	sims := make([]*Sim, 0)
	i := nTenant - 1
	for _, n := range nnodes {
		for _, npm := range npms {
			w := newWorld(n, nTenant, npm, ls, nTick, policyBidMore, 0.5)
			s := runSim(w)
			s.stats()
			// s.tenants[i].stats()
			sims = append(sims, s)
		}
	}
	r0 := float64(sims[0].tenants[i].nevict) / float64(sims[1].tenants[i].nevict)
	r1 := float64(sims[2].tenants[i].nevict) / float64(sims[3].tenants[i].nevict)

	r00 := float64(sims[0].mgr.nevict) / float64(sims[1].mgr.nevict)
	r11 := float64(sims[2].mgr.nevict) / float64(sims[3].mgr.nevict)

	fmt.Printf("%f %f %f %f\n", r0, r1, r00, r11)
	assert.True(t, r0 > 2)
	assert.True(t, r1 > 2)
}
