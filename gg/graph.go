package gg

import (
	"log"
)

type Thunk struct {
	hash  string
	deps  map[string]bool
	graph *Graph
}

type Graph struct {
	thunks map[string]*Thunk
	forced map[string]bool
}

func MakeGraph() *Graph {
	g := &Graph{}
	g.thunks = map[string]*Thunk{}
	g.forced = map[string]bool{}
	return g
}

func (g *Graph) AddThunk(hash string, deps []string) {
	// Ignore thunks which have already been added
	if _, ok := g.thunks[hash]; ok {
		return
	}
	t := &Thunk{}
	t.hash = hash
	t.graph = g
	t.deps = map[string]bool{}
	for _, d := range deps {
		t.deps[d] = false
	}
	g.thunks[hash] = t
}

func (g *Graph) GetThunks() []*Thunk {
	thunks := []*Thunk{}
	for len(g.thunks) > 0 {
		n_left := len(g.thunks)
		next := g.GetThunk()
		g.ForceThunk(next.hash)
		thunks = append(thunks, next)
		if n_left == len(g.thunks) {
			log.Fatalf("Couldn't remove thunk, %v left, g=%v\n", len(g.thunks), g)
		}
	}
	return thunks
}

func (g *Graph) GetThunk() *Thunk {
	var hash string
	for h, t := range g.thunks {
		if t.isRunnable() {
			hash = h
			break
		}
	}
	t := g.thunks[hash]
	delete(g.thunks, hash)
	return t
}

func (g *Graph) ForceThunk(hash string) {
	g.forced[hash] = true
}

func (t *Thunk) isRunnable() bool {
	for h, _ := range t.deps {
		if !t.graph.forced[h] {
			return false
		}
	}
	return true
}
