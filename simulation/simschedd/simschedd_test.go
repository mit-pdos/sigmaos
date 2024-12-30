package simmsched

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

type TrealmSmall struct {
	id      TrealmId
	poisson *distuv.Poisson
}

func uniform(r *rand.Rand) uint64 {
	return (rand.Uint64() % MAX_SERVICE_TIME) + 1
}

// nmsched ticks available per world tick; divide on average equally
// across realms.
func newTrealmSmall(id TrealmId, nmsched, nrealm int) *TrealmSmall {
	lambda := AVG_ARRIVAL_RATE_SMALL * (float64(nmsched) / float64(nrealm))
	return &TrealmSmall{id: id, poisson: &distuv.Poisson{Lambda: lambda}}
}

func (r *TrealmSmall) Id() TrealmId {
	return r.id
}

func (r *TrealmSmall) genLoad(rand *rand.Rand) []*Proc {
	nproc := int(r.poisson.Rand())
	procs := make([]*Proc, nproc)
	for i := 0; i < nproc; i++ {
		t := Tftick(0.95) // Ttick(uniform(rand))
		m := Tmem(1)      // Tmem(uniform(rand))
		procs[i] = newProc(t, m, r.id)
	}
	return procs
}

type TrealmBig struct {
	id      TrealmId
	nmsched int
	nrealm  int
	poisson *distuv.Poisson
	extra   bool
}

func newTrealmBig(id TrealmId, nmsched, nrealm int, extra bool) *TrealmBig {
	lambda := AVG_ARRIVAL_RATE_BIG * (float64(nmsched) / float64(nrealm))
	return &TrealmBig{id: id, poisson: &distuv.Poisson{Lambda: lambda}, extra: extra}
}

func (r *TrealmBig) Id() TrealmId {
	return r.id
}

func (r *TrealmBig) genLoad(rand *rand.Rand) []*Proc {
	nproc := int(r.poisson.Rand())
	procs := make([]*Proc, nproc)
	for i := 0; i < nproc; i++ {
		t := Tftick(MAX_SERVICE_TIME)
		if r.extra {
			t = t * 100
		}
		m := Tmem(MAX_MEM)
		procs[i] = newProc(t, m, r.id)
	}
	return procs
}

// <nreals> small realms
func newConfig(nProcQ, nSchedd, nrealm int) *World {
	w := newWorld(nProcQ, nSchedd)
	for i := 0; i < nrealm; i++ {
		r := newTrealmSmall(TrealmId(i), nSchedd, nrealm)
		w.addRealm(r)
	}
	return w
}

// zero or more small realms with one big realm
func newConfigBig(nProcQ, nSchedd, nrealm int, together, extra bool) *World {
	w := newWorld(nProcQ, nSchedd)
	if together {
		for i := 0; i < nrealm-1; i++ {
			w.addRealm(newTrealmSmall(TrealmId(i), nSchedd, nrealm))
		}
	}
	w.addRealm(newTrealmBig(TrealmId(nrealm-1), nSchedd, nrealm, extra))
	return w
}

func TestRunOneRealmSmall(t *testing.T) {
	w := newConfig(1, 1, 1)
	for i := 0; i < NTICK; i++ {
		w.Tick()
	}
}

func TestOneRealmBig(t *testing.T) {
	w := newConfigBig(1, 1, 1, true, false)
	for i := 0; i < NTICK; i++ {
		w.Tick()
	}
}

func TestRunSmallBig1(t *testing.T) {
	w := newConfigBig(1, 1, 2, true, false)
	for i := 0; i < NTICK; i++ {
		w.Tick()
	}
}

func TestRunSmallBigN(t *testing.T) {
	w := newConfigBig(1, 2, 2, true, false)
	for i := 0; i < NTICK; i++ {
		w.Tick()
	}
}

func TestRunSmallBigDelay(t *testing.T) {
	w := newConfigBig(1, 2, 1, false, true)
	for i := 0; i < NTICK; i++ {
		w.Tick()
		if i == 100 {
			w.addRealm(newTrealmSmall(TrealmId(1), 2, 2))
		}
	}
}

func TestRunSmallOneRealmTwoSchedd(t *testing.T) {
	w := newConfig(1, 2, 1)
	for i := 0; i < NTICK; i++ {
		w.Tick()
	}
}

func TestRunSmallTwoRealmTwoProcq(t *testing.T) {
	w := newConfig(2, 1, 2)
	for i := 0; i < NTICK; i++ {
		w.Tick()
	}
}

func TestRunSmallTwoAll(t *testing.T) {
	w := newConfig(2, 2, 2)
	for i := 0; i < NTICK; i++ {
		w.Tick()
	}
}
