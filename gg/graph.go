package gg

import (
	"fmt"

	db "ulambda/debug"
)

type Thunk struct {
	hash        string
	deps        map[string]bool
	outputFiles []string
	graph       *Graph
}

type Graph struct {
	thunks map[string]*Thunk
	forced map[string]bool
	swaps  map[string]string
}

func MakeGraph() *Graph {
	g := &Graph{}
	g.thunks = map[string]*Thunk{}
	g.forced = map[string]bool{}
	g.swaps = map[string]string{}
	return g
}

func (g *Graph) AddThunk(hash string, deps []string, outputFiles []string) {
	// Ignore thunks which have already been added
	if _, ok := g.thunks[hash]; ok {
		return
	}
	t := &Thunk{}
	t.hash = hash
	t.graph = g
	t.deps = map[string]bool{}
	for _, d := range deps {
		// Take into account exit dep swaps
		updatedDep := d
		if swap, ok := g.swaps[d]; ok {
			updatedDep = swap
		}
		// Only add exit deps for thunks which haven't been forced...
		if forced, ok := g.forced[updatedDep]; !ok || !forced {
			t.deps[updatedDep] = false
		}
	}
	t.outputFiles = outputFiles
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
			db.DFatalf("Couldn't remove thunk, %v left, g=%v\n", len(g.thunks), g)
		}
	}
	return thunks
}

func (g *Graph) GetThunk() *Thunk {
	for h, t := range g.thunks {
		if t.isRunnable() {
			delete(g.thunks, h)
			return t
		}
	}
	return nil
}

func (g *Graph) GetRunnableThunks() []*Thunk {
	thunks := []*Thunk{}
	for {
		next := g.GetThunk()
		if next == nil {
			break
		}
		thunks = append(thunks, next)
	}
	return thunks
}

func (g *Graph) ForceThunk(hash string) {
	g.forced[hash] = true
}

func (g *Graph) SwapDeps(from string, to string) {
	// Update swap pointers
	g.swaps[from] = to
	for f, t := range g.swaps {
		if t == from {
			g.swaps[f] = to
		}
	}
	for _, t := range g.thunks {
		if _, ok := t.deps[from]; ok {
			delete(t.deps, from)
			t.deps[to] = false
		}
	}
}

func (t *Thunk) isRunnable() bool {
	for h, _ := range t.deps {
		if !t.graph.forced[h] {
			return false
		}
	}
	return true
}

func (t *Thunk) String() string {
	return fmt.Sprintf("{ hash:%v deps:%v outputFiles:%v }", t.hash, t.deps, t.outputFiles)
}

func (g *Graph) String() string {
	thunks := []*Thunk{}
	forced := []string{}
	for _, t := range g.thunks {
		thunks = append(thunks, t)
	}
	for h, _ := range g.forced {
		forced = append(forced, h)
	}
	return fmt.Sprintf("{ thunks:%v forced:%v }", thunks, forced)
}
