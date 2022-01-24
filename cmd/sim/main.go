package main

import (
	"fmt"
	"math"
	"math/rand"
	"time"
)

const (
	NNODE = 100

	// BE are between 1 and 100 ticks
	MAXTICK = 100

	// too get the whole cluster to LC lambdas
	NPEAK   = NNODE - 1
	NTICKLC = NNODE

	NTRIAL = 1 // 10
)

// Node may run several lambdas with preemption
type Node struct {
	ls []*Lambda
}

type Lambda struct {
	nTick float64
	load  int
	lc    bool
}

type Sim struct {
	time    uint64
	nodes   []*Node
	lc      []*Lambda
	lcNwait uint64
	nlc     int
	nbe     int
	rand    *rand.Rand
	peak    int

	// paramaters
	maxutil float64
	preempt bool
}

func zipf(r *rand.Rand) uint64 {
	z := rand.NewZipf(r, 2.0, 1.0, MAXTICK-1)
	return z.Uint64() + 1
}

func uniform(r *rand.Rand) uint64 {
	return (rand.Uint64() % MAXTICK) + 1
}

func mkLambda(lc bool, ntick uint64) *Lambda {
	l := &Lambda{}
	if lc {
		l.lc = lc
	}
	l.nTick = float64(ntick)
	return l
}

func (l *Lambda) String() string {
	return fmt.Sprintf("n %.2f(%v)", l.nTick, l.lc)
}

func (n *Node) String() string {
	return fmt.Sprintf("ls %v", n.ls)
}

func mkSim() *Sim {
	sim := &Sim{}
	sim.rand = rand.New(rand.NewSource(time.Now().UnixNano()))
	sim.nodes = make([]*Node, NNODE)
	for i := 0; i < NNODE; i++ {
		sim.nodes[i] = &Node{}
	}
	sim.lc = make([]*Lambda, 0)
	sim.lc = append(sim.lc, mkLambda(true, math.MaxUint64))
	sim.runLC(false)
	sim.peak = NPEAK
	return sim
}

func (sim *Sim) util() float64 {
	return float64(sim.nbe+sim.nlc) / float64(NNODE)
}

func (sim *Sim) Print() {
	fmt.Printf("%v: peak %v nlc %v nodes %v qlc %v util %v\n",
		sim.time, sim.peak, sim.nlc, sim.nodes,
		len(sim.lc), sim.util())
}

func (sim *Sim) incLoad() {
	sim.lc = append(sim.lc, mkLambda(true, NTICKLC))
	sim.nlc += 1
}

func (sim *Sim) getLCLambda() *Lambda {
	var l *Lambda
	l, sim.lc = sim.lc[0], sim.lc[1:]
	return l
}

func (sim *Sim) getBELambda() *Lambda {
	// return mkLambda(false, zipf(sim.rand))
	return mkLambda(false, uniform(sim.rand))
}

// Add LCs to nodes who aren't running anything or add to a node that
// runs a BE lambda (if sharing)
func (sim *Sim) runLC(share bool) {
	i := 0
	for _, n := range sim.nodes {
		if i >= len(sim.lc) {
			break
		}
		if len(n.ls) == 0 {
			n.ls = append(n.ls, sim.lc[i])
			i++
		}
		if share && len(n.ls) == 1 && !n.ls[0].lc {
			n.ls = append(n.ls, sim.lc[i])
			i++
		}
	}
	sim.lc = sim.lc[i:]
}

func (sim *Sim) tick() bool {
	for _, n := range sim.nodes {
		for i := 0; i < len(n.ls); i++ {
			l := n.ls[i]
			l.nTick -= float64(1) / float64(len(n.ls))
			if l.nTick <= 0 {
				if l.lc {
					sim.nlc--
				} else {
					sim.nbe--
				}
				n.ls = append(n.ls[:i], n.ls[i+1:]...)
				i--
			}
		}
		if len(n.ls) == 0 {
			if len(sim.lc) > 0 {
				n.ls = append(n.ls, sim.getLCLambda())
			} else if sim.util() < sim.maxutil {
				n.ls = append(n.ls, sim.getBELambda())
				sim.nbe++
			}
		}
	}
	if len(sim.lc) > 0 {
		// fmt.Printf("%v: wait %v\n", sim.time, len(sim.lc))
		sim.lcNwait += uint64(len(sim.lc))
		// sim.Print()

	}
	if sim.time >= 50 && sim.peak > 0 {
		sim.peak--
		sim.incLoad()
		sim.runLC(false)
		if sim.preempt {
			sim.runLC(true)
		}
	}
	return sim.peak == 0 && sim.nlc == 0
}

func runSim(util float64, preempt bool) {
	t := uint64(0)
	w := uint64(0)
	for i := 0; i < NTRIAL; i++ {
		sim := mkSim()
		sim.maxutil = util
		sim.preempt = preempt
		stop := false
		for !stop {
			stop = sim.tick()
			sim.time += 1
		}
		// fmt.Printf("nwait %v\n", sim.lcNwait)
		t += sim.time
		w += sim.lcNwait
	}
	n := float64(NTRIAL)
	fmt.Printf("Util %f preempt %v ticks %f nWait %f \n", util,
		preempt, float64(t)/n, float64(w)/n)
}

func main() {
	runSim(0.5, false)
	runSim(0.5, true)
	runSim(0.9, false)
	runSim(0.9, true)
}
