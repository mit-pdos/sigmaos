package simschedd

import (
	"math/rand"
	"testing"

	"gonum.org/v1/gonum/stat/distuv"
)

const (
	NTICK = 1000

	// proc arrival rates
	AVG_ARRIVAL_RATE_SMALL float64 = 1.0 // per tick (with 1 tick per proc)
	AVG_ARRIVAL_RATE_BIG   float64 = 0.1 // per tick (with 10 ticks per proc)
)

type Trealm struct {
	id      TrealmId
	poisson *distuv.Poisson
}

func uniform(r *rand.Rand) uint64 {
	return (rand.Uint64() % MAX_SERVICE_TIME) + 1
}

// nschedd ticks available per world tick; divide on average equally
// across realms.
func newTrealm(id TrealmId, nschedd, nrealm int) *Trealm {
	lambda := AVG_ARRIVAL_RATE_SMALL * (float64(nschedd) / float64(nrealm))
	return &Trealm{id: id, poisson: &distuv.Poisson{Lambda: lambda}}
}

func (r *Trealm) Id() TrealmId {
	return r.id
}

func (r *Trealm) genLoad(rand *rand.Rand) []*Proc {
	nproc := int(r.poisson.Rand())
	procs := make([]*Proc, nproc)
	for i := 0; i < nproc; i++ {
		t := Ttick(1) // Ttick(uniform(rand))
		m := Tmem(1)  // Tmem(uniform(rand))
		procs[i] = newProc(t, m, r.id)
	}
	return procs
}

type TrealmBig struct {
	id      TrealmId
	nschedd int
	nrealm  int
	poisson *distuv.Poisson
}

func newTrealmBig(id TrealmId, nschedd, nrealm int) *TrealmBig {
	lambda := AVG_ARRIVAL_RATE_BIG / float64(nrealm)
	return &TrealmBig{id: id, poisson: &distuv.Poisson{Lambda: lambda}}
}

func (r *TrealmBig) Id() TrealmId {
	return r.id
}

func (r *TrealmBig) genLoad(rand *rand.Rand) []*Proc {
	nproc := int(r.poisson.Rand())
	procs := make([]*Proc, nproc)
	for i := 0; i < nproc; i++ {
		t := Ttick(MAX_SERVICE_TIME)
		m := Tmem(MAX_MEM)
		procs[i] = newProc(t, m, r.id)
	}
	return procs
}

func newConfig(nProcQ, nSchedd, nrealm int) *World {
	w := newWorld(nProcQ, nSchedd)
	for i := 0; i < nrealm; i++ {
		r := newTrealm(TrealmId(i), nSchedd, nrealm)
		w.addRealm(r)
	}
	return w
}

func newConfigBig(nProcQ, nSchedd, nrealm int) *World {
	w := newWorld(nProcQ, nSchedd)
	for i := 0; i < nrealm-1; i++ {
		w.addRealm(newTrealm(TrealmId(i), nSchedd, nrealm-1))
	}
	w.addRealm(newTrealmBig(TrealmId(nrealm-1), nSchedd, nrealm))
	return w
}

func TestRunOneRealmSmall(t *testing.T) {
	w := newConfig(1, 1, 1)
	for i := 0; i < NTICK; i++ {
		w.Tick()
	}
}

func TestOneRealmBig(t *testing.T) {
	w := newConfigBig(1, 1, 1)
	for i := 0; i < NTICK; i++ {
		w.Tick()
	}
}

func TestRunSmallBig(t *testing.T) {
	w := newConfigBig(1, 1, 2)
	for i := 0; i < NTICK; i++ {
		w.Tick()
	}
}

func TestRunOneRealmTwoSchedd(t *testing.T) {
	w := newConfig(1, 2, 1)
	for i := 0; i < NTICK; i++ {
		w.Tick()
	}
}

func TestRunTwoRealmOneSchedd(t *testing.T) {
	w := newConfig(1, 1, 2)
	for i := 0; i < NTICK; i++ {
		w.Tick()
	}
}

func TestRunTwoRealmTwoProcq(t *testing.T) {
	w := newConfig(2, 1, 2)
	for i := 0; i < NTICK; i++ {
		w.Tick()
	}
}

func TestRunTwoAll(t *testing.T) {
	w := newConfig(2, 2, 2)
	for i := 0; i < NTICK; i++ {
		w.Tick()
	}
}
