package main

import (
	"fmt"
	"math/rand"
	"time"

	"gonum.org/v1/gonum/stat/distuv"
)

const (
	NNODE   = 2
	NTENANT = 1
	NTRIAL  = 1 // 10

	// Assume TICK is 1ms
	NTICK                    = 100
	AVG_ARRIVAL_RATE float64 = 0.1 // 1 req per 10ms
	MAX_SERVICE_TIME         = 5   // 1 ms
)

func zipf(r *rand.Rand) uint64 {
	z := rand.NewZipf(r, 2.0, 1.0, MAX_SERVICE_TIME-1)
	return z.Uint64() + 1
}

func uniform(r *rand.Rand) uint64 {
	return (rand.Uint64() % MAX_SERVICE_TIME) + 1
}

type Proc struct {
	nTick uint64
}

func (p *Proc) String() string {
	return fmt.Sprintf("n %d", p.nTick)
}

type Node struct {
	proc   *Proc
	price  float64
	tenant *Tenant
}

func (n *Node) String() string {
	return fmt.Sprintf("{proc %v price %.2f %p}", n.proc, n.price, n.tenant)
}

type Tenant struct {
	budget  float64
	procs   []*Proc
	nodes   []*Node
	sim     *Sim
	nproc   int
	nnode   int
	maxnode int
	nwork   int
}

func (t Tenant) String() string {
	s := fmt.Sprintf("{budget %.2f nproc %d nnode %d procq: [", t.budget, t.nproc, t.nnode)
	for _, p := range t.procs {
		s += fmt.Sprintf("{%v} ", p)
	}
	s += fmt.Sprintf("] nodes: [")
	for _, n := range t.nodes {
		s += fmt.Sprintf("%v ", n)
	}
	return s + "]}"
}

func (t *Tenant) tick() {
	nproc := int(t.sim.poisson.Rand())
	for i := 0; i < nproc; i++ {
		t.procs = append(t.procs, t.sim.mkProc())
	}
	t.nproc += nproc
	t.schedule()
	if len(t.procs) > 0 {
		if n := t.sim.mgr.bidNode(t, 0.0); n != nil {
			t.nodes = append(t.nodes, n)
			t.schedule()
		}
	}
	t.nnode += len(t.nodes)
	if len(t.nodes) > t.maxnode {
		t.maxnode = len(t.nodes)
	}
}

func (t *Tenant) schedule() {
	for _, n := range t.nodes {
		if len(t.procs) == 0 {
			return
		}
		if n.proc == nil {
			n.proc = t.procs[0]
			t.procs = t.procs[1:]
		}
	}
}

func (t *Tenant) stats() {
	n := float64(NTICK)
	fmt.Printf("%p: lambda %.2f avg nnode %.2f max node %d nwork %d load %.2f\n", t, float64(t.nproc)/n, float64(t.nnode)/n, t.maxnode, t.nwork, float64(t.nwork)/float64(t.nnode))
}

type Mgr struct {
	price float64
	nodes *[NNODE]Node
}

func mkMgr(nodes *[NNODE]Node) *Mgr {
	m := &Mgr{}
	m.nodes = nodes
	return m
}

func (m *Mgr) String() string {
	s := fmt.Sprintf("{mgr price %.2f nodes:", m.price)
	for i, _ := range m.nodes {
		s += fmt.Sprintf("{%v} ", m.nodes[i])
	}
	return s + "}"
}

func (m *Mgr) bidNode(t *Tenant, b float64) *Node {
	for i, _ := range m.nodes {
		n := &m.nodes[i]
		if n.tenant == nil {
			n.tenant = t
			n.price = b
			fmt.Printf("bidNode -> %v\n", n)
			return n
		}
	}
	fmt.Printf("bidNode %p failed\n", t)
	return nil
}

type Sim struct {
	time    uint64
	nodes   [NNODE]Node
	tenants [NTENANT]Tenant
	rand    *rand.Rand
	mgr     *Mgr
	poisson *distuv.Poisson
}

func mkSim() *Sim {
	sim := &Sim{}
	sim.rand = rand.New(rand.NewSource(time.Now().UnixNano()))
	sim.mgr = mkMgr(&sim.nodes)
	sim.poisson = &distuv.Poisson{Lambda: AVG_ARRIVAL_RATE}
	for i := 0; i < NTENANT; i++ {
		t := &sim.tenants[i]
		t.procs = make([]*Proc, 0)
		t.sim = sim
	}
	return sim
}

func (sim *Sim) Print() {
	fmt.Printf("%v: nodes %v\n", sim.time, sim.nodes)
}

func (sim *Sim) mkProc() *Proc {
	p := &Proc{}
	// p.nTick = zipf(sim.rand)
	p.nTick = uniform(sim.rand)
	return p
}

func (sim *Sim) tickTenants() {
	for i, _ := range sim.tenants {
		t := &sim.tenants[i]
		t.tick()
	}
}

func (sim *Sim) tick() {
	sim.tickTenants()
	fmt.Printf("tick tenants %v\n", sim.tenants)
	for i, _ := range sim.nodes {
		n := &sim.nodes[i]
		if n.proc != nil {
			n.proc.nTick--
			n.tenant.nwork++
			if n.proc.nTick == 0 {
				n.proc = nil
			}
		}
	}
}

func runSim() {
	for i := 0; i < NTRIAL; i++ {
		sim := mkSim()
		for ; sim.time < NTICK; sim.time++ {
			sim.tick()
		}
		for _, t := range sim.tenants {
			t.stats()
		}
	}
}

func main() {
	runSim()
}
