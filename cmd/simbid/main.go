package main

import (
	"fmt"
	"math/rand"
	"time"

	"gonum.org/v1/gonum/stat/distuv"
)

const (
	NNODE   = 10
	NTENANT = 100
	NTRIAL  = 1 // 10

	NTICK                    = 100
	AVG_ARRIVAL_RATE float64 = 0.1 // per tick
	MAX_SERVICE_TIME         = 5   // in ticks
	MAX_BID          float64 = 1.0 // per tick
)

func zipf(r *rand.Rand) uint64 {
	z := rand.NewZipf(r, 2.0, 1.0, MAX_SERVICE_TIME-1)
	return z.Uint64() + 1
}

func uniform(r *rand.Rand) uint64 {
	return (rand.Uint64() % MAX_SERVICE_TIME) + 1
}

//
// Tenants runs procs.  At each tick, each tenant creates new procs
// based AVG_ARRIVAL_RATE.  Each proc runs for nTick, following either
// uniform or zipfian distribution.
//

type Proc struct {
	nLength uint64  // in ticks
	nTick   uint64  // #ticks remaining
	cost    float64 // cost for this proc
}

func (p *Proc) String() string {
	return fmt.Sprintf("n %d", p.nTick)
}

//
// Computing nodes that the manager allocates to tenants.  Each node
// runs one proc or is idle.
//

type Node struct {
	proc   *Proc
	price  float64 // the price for a tick
	tenant *Tenant
}

func (n *Node) String() string {
	return fmt.Sprintf("{proc %v price %.2f %p}", n.proc, n.price, n.tenant)
}

func (n *Node) reallocate(to *Tenant, b float64) {
	fmt.Printf("reallocate %v(%p) to %p\n", n, n, to)
	n.tenant.evict(n)
	n.tenant = to
	n.price = b
}

//
// Tenants run procs on the nodes allocated to them by the mgr. If
// they have more procs to run than available nodes, tenant bids for
// more nodes up till its maxbid.
//

type Tenant struct {
	poisson  *distuv.Poisson
	maxbid   float64
	procs    []*Proc
	nodes    []*Node
	sim      *Sim
	nproc    int // sum of proc ticks
	nnode    int // sum of node ticks
	maxnode  int
	nwork    uint64  // sum of # ticks running a proc
	cost     float64 // cost for nwork ticks
	nwait    uint64  // sum of # ticks waiting to be run
	nevict   int     // # evictions
	nwasted  uint64  // sum # ticks wasted because of eviction
	sunkCost float64 // the cost of the wasted ticks
}

func (t *Tenant) String() string {
	s := fmt.Sprintf("{nproc %d nnode %d procq: [", t.nproc, t.nnode)
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
	nproc := int(t.poisson.Rand())
	for i := 0; i < nproc; i++ {
		t.procs = append(t.procs, t.sim.mkProc())
	}
	t.nproc += nproc
	t.schedule()

	// if we still have procs queued for execution, bid for a new
	// node, increasing the bid until mgr accepts or bid reaches max
	// bid.
	bid := float64(0.0)
	if t == &t.sim.tenants[0] {
		bid = float64(0.5)
	}
	for len(t.procs) > 0 && bid <= t.maxbid {
		if n := t.sim.mgr.bidNode(t, bid); n != nil {
			// fmt.Printf("%p: bid accepted at %.2f\n", t, bid)
			t.nodes = append(t.nodes, n)
			t.schedule()
		} else {
			bid += 0.1
		}
	}

	t.freeIdle()

	t.nnode += len(t.nodes)
	if len(t.nodes) > t.maxnode {
		t.maxnode = len(t.nodes)
	}
	t.nwait += uint64(len(t.procs))
	t.charge()
}

func (t *Tenant) freeIdle() {
	for i := 0; i < len(t.nodes); i++ {
		n := t.nodes[i]
		if n.proc == nil {
			t.nodes = append(t.nodes[0:i], t.nodes[i+1:]...)
			i--
			t.sim.mgr.yield(n)
		}
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

// Manager is taking away a node
func (t *Tenant) evict(n *Node) {
	if n.proc != nil {
		c := n.proc.nLength - n.proc.nTick // wasted ticks
		t.nevict++
		t.nwasted += c
		if c > 0 {
			t.sunkCost += n.proc.cost
		}
		n.proc = nil
	}
	for i, _ := range t.nodes {
		if t.nodes[i] == n {
			t.nodes = append(t.nodes[0:i], t.nodes[i+1:]...)
			return
		}
	}
	panic("evict")
}

func (t *Tenant) charge() {
	c := float64(0)
	for _, n := range t.nodes {
		if n.proc == nil {
			panic("charge")
		}
		n.proc.cost += n.price
		c += n.price
	}
	t.cost += c
	t.sim.mgr.revenue += c
}

func (t *Tenant) stats() {
	n := float64(NTICK)
	fmt.Printf("%p: l %.2f P/T %.2f maxN %d work %dT util %.2f nwait %dT #evict %dP (waste %dT) charge $%.2f sunk $%.2f tick $%.2f\n", t, float64(t.nproc)/n, float64(t.nnode)/n, t.maxnode, t.nwork, float64(t.nwork)/float64(t.nnode), t.nwait, t.nevict, t.nwasted, t.cost, t.sunkCost, float64(t.cost)/float64(t.nwork))
}

//
// Manager assigns nodes to tenants
//

type Mgr struct {
	price   float64
	nodes   *[NNODE]Node
	index   int
	revenue float64
	nwork   int
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

func (m *Mgr) stats() {
	n := NTICK * NNODE
	fmt.Printf("Mgr revenue %.2f avg rev/tick %.2f util %.2f\n", m.revenue, float64(m.revenue)/float64(m.nwork), float64(m.nwork)/float64(n))
}

func (m *Mgr) findFree(t *Tenant, b float64) *Node {
	for i, _ := range m.nodes {
		n := &m.nodes[i]
		if n.tenant == nil {
			n.tenant = t
			n.price = b
			return n
		}
	}
	return nil
}

func (m *Mgr) yield(n *Node) {
	fmt.Printf("yield %v(%p)\n", n, n)
	n.tenant = nil
}

func (m *Mgr) bidNode(t *Tenant, b float64) *Node {
	// fmt.Printf("bidNode %p %.2f\n", t, b)
	if n := m.findFree(t, b); n != nil {
		// fmt.Printf("bidNode -> unused %v\n", n)
		return n
	}
	// no unused nodes; look for a node with price lower than b
	// re-allocate it to tenant t, after given the old tenant a chance
	// to evict its proc from the node.
	s := m.index
	for {
		n := &m.nodes[m.index%len(m.nodes)]
		m.index = (m.index + 1) % len(m.nodes)
		if b > n.price && n.tenant != t {
			n.reallocate(t, b)
			return n
		}
		if m.index == s { // looped around; no lower priced node exists
			break
		}
	}
	return nil
}

//
// Run simulation
//

type Sim struct {
	time    uint64
	nodes   [NNODE]Node
	tenants [NTENANT]Tenant
	rand    *rand.Rand
	mgr     *Mgr
}

func mkSim() *Sim {
	sim := &Sim{}
	sim.rand = rand.New(rand.NewSource(time.Now().UnixNano()))
	sim.mgr = mkMgr(&sim.nodes)
	for i := 0; i < NTENANT; i++ {
		t := &sim.tenants[i]
		if i == 0 {
			t.poisson = &distuv.Poisson{Lambda: 10 * AVG_ARRIVAL_RATE}
		} else {
			t.poisson = &distuv.Poisson{Lambda: AVG_ARRIVAL_RATE}
		}
		t.procs = make([]*Proc, 0)
		t.sim = sim
		t.maxbid = MAX_BID
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
	p.nLength = p.nTick
	return p
}

func (sim *Sim) tickTenants(tick uint64) {
	for i, _ := range sim.tenants {
		sim.tenants[i].tick()
	}
	fmt.Printf("tick %d:", tick)
	for i, _ := range sim.tenants {
		t := &sim.tenants[i]
		if len(t.procs) > 0 || len(t.nodes) > 0 {
			fmt.Printf("\n%p: %v", t, t)
		}
	}
	fmt.Printf("\n")
}

func (sim *Sim) tick(tick uint64) {
	sim.tickTenants(tick)
	for i, _ := range sim.nodes {
		n := &sim.nodes[i]
		if n.proc != nil {
			n.proc.nTick--
			n.tenant.nwork++
			sim.mgr.nwork++
			if n.proc.nTick == 0 {
				n.proc = nil
			}
		}
	}
}

func main() {
	for i := 0; i < NTRIAL; i++ {
		sim := mkSim()
		for ; sim.time < NTICK; sim.time++ {
			sim.tick(sim.time)
		}
		for i, _ := range sim.tenants {
			sim.tenants[i].stats()
		}
		sim.mgr.stats()
	}
}
