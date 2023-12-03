package simschedd

import (
	"math/rand"
	"testing"

	"gonum.org/v1/gonum/stat/distuv"
)

const (
	NTICK = 1000

	AVG_ARRIVAL_RATE float64 = 0.2 // per tick
)

type Trealm struct {
	id      TrealmId
	poisson *distuv.Poisson
}

func newTrealm(id TrealmId, nschedd, nrealm int) *Trealm {
	lambda := AVG_ARRIVAL_RATE * (float64(nschedd) / float64(nrealm))
	r := &Trealm{id: id, poisson: &distuv.Poisson{Lambda: lambda}}
	return r
}

func (r *Trealm) Id() TrealmId {
	return r.id
}

func (r *Trealm) genLoad(rand *rand.Rand) []*Proc {
	nproc := int(r.poisson.Rand())
	procs := make([]*Proc, nproc)
	for i := 0; i < nproc; i++ {
		t := Ttick(uniform(rand))
		m := Tmem(uniform(rand))
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

func TestRunOneRealmSimple(t *testing.T) {
	w := newConfig(1, 1, 1)
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
