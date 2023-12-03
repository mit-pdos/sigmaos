package simschedd

import (
	"testing"
)

const N = 1000

func TestRunOneRealmSimple(t *testing.T) {
	w := newWorld(1, 1, 1)
	for i := 0; i < N; i++ {
		w.Tick()
	}
}

func TestRunOneRealmTwoSchedd(t *testing.T) {
	w := newWorld(1, 2, 1)
	for i := 0; i < N; i++ {
		w.Tick()
	}
}

func TestRunTwoRealmOneSchedd(t *testing.T) {
	w := newWorld(1, 1, 2)
	for i := 0; i < N; i++ {
		w.Tick()
	}
}

func TestRunTwoRealmTwoProcq(t *testing.T) {
	w := newWorld(2, 1, 2)
	for i := 0; i < N; i++ {
		w.Tick()
	}
}

func TestRunTwoAll(t *testing.T) {
	w := newWorld(2, 2, 2)
	for i := 0; i < N; i++ {
		w.Tick()
	}
}
