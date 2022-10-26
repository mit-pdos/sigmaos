package main

import (
	"fmt"
	"math/rand"
	"reflect"
	"runtime"
	"sort"
	"time"

	"gonum.org/v1/gonum/stat/distuv"
)

const (
	NNODE   = 35
	NTENANT = 100
	NTRIAL  = 1 // 10
	DEBUG   = false

	NTICK                    = 100
	AVG_ARRIVAL_RATE float64 = 0.1 // per tick
	MAX_SERVICE_TIME         = 5   // in ticks
	// DURATION                 = NTICK
	PRICE_ONDEMAND Price = 0.00000001155555555555 // per ms for 1h on AWS
	PRICE_SPOT     Price = 0.00000000347222222222 // per ms
	BID_INCREMENT        = 0.000000000001
	MAX_BID        Price = 3 * PRICE_SPOT
)

func zipf(r *rand.Rand) uint64 {
	z := rand.NewZipf(r, 2.0, 1.0, MAX_SERVICE_TIME-1)
	return z.Uint64() + 1
}

func uniform(r *rand.Rand) uint64 {
	return (rand.Uint64() % MAX_SERVICE_TIME) + 1
}

type Tpolicy func(*Tenant, Price) *Bid

//
// Price
//

type Price float64

func (p Price) String() string {
	return fmt.Sprintf("$%.12f", p)
}

//
// Bid
//

type Bid struct {
	tenant *Tenant
	bids   []Price // one bid per node
}

func (b *Bid) String() string {
	return fmt.Sprintf("{t: %p %v %d}", b.tenant, b.bids, len(b.bids))
}

func mkBid(t *Tenant, bs []Price) *Bid {
	return &Bid{t, bs}
}

type Bids []*Bid

func (bs *Bids) PopHighest(rand *rand.Rand) (*Tenant, Price) {
	bid := Price(0.0)

	if len(*bs) == 0 {
		return nil, bid
	}

	// find highest bid
	for _, b := range *bs {
		if b.bids[0] > bid {
			bid = b.bids[0]
		}
	}

	// find bidders for highest
	bidders := make([]int, 0)
	for i, b := range *bs {
		if b.bids[0] == bid {
			bidders = append(bidders, i)
		}
	}

	// pick a random higest
	n := bidders[int(rand.Uint64()%uint64(len(bidders)))]
	t := (*bs)[n].tenant

	// remove higest bid
	(*bs)[n].bids = (*bs)[n].bids[1:]
	if len((*bs)[n].bids) == 0 {
		*bs = append((*bs)[:n], (*bs)[n+1:]...)
	}
	return t, bid
}

//
// Tenants runs procs.  At each tick, each tenant creates new procs
// based AVG_ARRIVAL_RATE.  Each proc runs for nTick, following either
// uniform or zipfian distribution.
//

type Proc struct {
	nLength uint64 // in ticks
	nTick   uint64 // #ticks remaining
	cost    Price  // cost for this proc
}

func (p *Proc) String() string {
	return fmt.Sprintf("n %d", p.nTick)
}

func mkProc(rand *rand.Rand) *Proc {
	p := &Proc{}
	// p.nTick = zipf(rand)
	p.nTick = uniform(rand)
	p.nLength = p.nTick
	return p
}

//
// Computing nodes that the manager allocates to tenants.  Each node
// runs one proc or is idle.
//

type Node struct {
	proc   *Proc
	price  Price // the price for a tick
	tenant *Tenant
}

func (n *Node) String() string {
	return fmt.Sprintf("{%p: proc %v price %v t %p}", n, n.proc, n.price, n.tenant)
}

type Nodes []*Node

func (ns *Nodes) remove(n1 *Node) *Node {
	for i, n := range *ns {
		if n == n1 {
			*ns = append((*ns)[:i], (*ns)[i+1:]...)
			return n
		}
	}
	return nil
}

func (ns *Nodes) findFree() *Node {
	for i, n := range *ns {
		if n.tenant == nil {
			*ns = append((*ns)[:i], (*ns)[i+1:]...)
			return n
		}
	}
	return nil
}

func (ns *Nodes) findVictim(t *Tenant, bid Price) *Node {
	for _, n := range *ns {
		if n.tenant != t && bid > n.price {
			return n
		}
	}
	return nil
}

func (ns *Nodes) isPresent(nn *Node) bool {
	for _, n := range *ns {
		if n == nn {
			return true
		}
	}
	return false
}

//
// Tenants run procs on the nodes allocated to them by the mgr. If
// they have more procs to run than available nodes, tenant bids for
// more nodes.
//

type Tenant struct {
	poisson  *distuv.Poisson
	procs    []*Proc
	nodes    Nodes
	nbid     int
	ngrant   int
	sim      *Sim
	nproc    int // sum of proc ticks
	nnode    int // sum of node ticks
	maxnode  int
	nwork    uint64 // sum of # ticks running a proc
	cost     Price  // cost for nwork ticks
	nwait    uint64 // sum of # ticks waiting to be run
	nevict   int    // # evictions
	nwasted  uint64 // sum # ticks wasted because of eviction
	sunkCost Price  // the cost of the wasted ticks
	policy   Tpolicy
}

func (t *Tenant) String() string {
	s := fmt.Sprintf("{nproc %d nnode %d procq (%d): [", t.nproc, t.nnode, len(t.procs))
	for _, p := range t.procs {
		s += fmt.Sprintf("{%v} ", p)
	}
	s += fmt.Sprintf("] nodes (%d): [", len(t.nodes))
	for _, n := range t.nodes {
		s += fmt.Sprintf("%v ", n)
	}
	return s + "]}"
}

// New procs "arrive" based on Poisson distribution. Schedule queued
// procs on the available nodes, and release nodes we don't use.
func (t *Tenant) genProcs() (int, uint64) {
	nproc := int(t.poisson.Rand())
	len := uint64(0)
	for i := 0; i < nproc; i++ {
		p := mkProc(t.sim.rand)
		len += p.nLength
		t.procs = append(t.procs, p)
	}
	t.nproc += nproc
	t.schedule()
	t.yieldIdle()
	return nproc, len
}

func policyBigMore(t *Tenant, last Price) *Bid {
	bids := make([]Price, 0)
	if t == &t.sim.tenants[0] && len(t.nodes) == 0 {
		// very first bid for tenant 0, which has a higher load grab
		// one high-priced node to sustant the expected load of 1.
		bids = append(bids, PRICE_ONDEMAND)
		for i := 0; i < len(t.procs)-1; i++ {
			bids = append(bids, last)
		}
	} else {
		for i := 0; i < len(t.procs); i++ {
			bids = append(bids, last)
		}
	}
	return mkBid(t, bids)
}

// Bid one up from last, the lowest winning bid
func policyLast(t *Tenant, last Price) *Bid {
	bids := make([]Price, 0)
	for i := 0; i < len(t.procs); i++ {
		bids = append(bids, last+BID_INCREMENT)
	}
	return mkBid(t, bids)
}

func policyFixed(t *Tenant, last Price) *Bid {
	bids := make([]Price, 0)
	for i := 0; i < len(t.procs); i++ {
		bids = append(bids, PRICE_SPOT)
	}
	return mkBid(t, bids)
}

// Bid for new nodes if we have queued procs.  last is avg succesful
// bid in the last round.
func (t *Tenant) bid(last Price) *Bid {
	t.ngrant = 0
	t.nbid = len(t.procs)
	if len(t.procs) > 0 {
		return t.policy(t, last)
	}
	return nil
}

// mgr grants a node
func (t *Tenant) grantNode(n *Node) {
	t.ngrant++
	t.nodes = append(t.nodes, n)
}

// After bidding, we may have received new nodes; use them.
func (t *Tenant) scheduleNodes() int {
	//if t.nbid > 0 && t.ngrant < t.nbid {
	//	fmt.Printf("%p: asked %d and received %d\n", t, t.nbid, t.ngrant)
	//}
	t.schedule()
	t.nnode += len(t.nodes)
	if len(t.nodes) > t.maxnode {
		t.maxnode = len(t.nodes)
	}
	t.nwait += uint64(len(t.procs))
	return len(t.procs)
}

func (t *Tenant) yieldIdle() {
	for i := 0; i < len(t.nodes); i++ {
		n := t.nodes[i]
		if n.proc == nil && n.price != PRICE_ONDEMAND {
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
		t.sim.mgr.nwasted += c
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
	fmt.Printf("%p: n not found %v\n", t, n)
	panic("evict")
}

func (t *Tenant) isPresent(n *Node) bool {
	return t.nodes.isPresent(n)
}

func (t *Tenant) stats() {
	n := float64(NTICK)
	fmt.Printf("%p: l %v P/T %.2f maxN %d work %dT util %.2f nwait %dT #evict %dP (waste %dT) charge %v sunk %v tick %v\n", t, float64(t.nproc)/n, float64(t.nnode)/n, t.maxnode, t.nwork, float64(t.nwork)/float64(t.nnode), t.nwait, t.nevict, t.nwasted, t.cost, t.sunkCost, t.cost/Price(t.nwork))
}

//
// Manager assigns nodes to tenants
//

type Mgr struct {
	sim     *Sim
	free    Nodes
	cur     Nodes
	index   int
	revenue Price
	nwork   int
	nidle   uint64
	nevict  uint64
	nwasted uint64
	last    Price // lowest bid accepted in last tick
	avgbid  Price // avg bid in last tick
	high    Price // highest bid in last tick
}

func mkMgr(sim *Sim) *Mgr {
	m := &Mgr{}
	m.sim = sim
	ns := make(Nodes, NNODE, NNODE)
	for i, _ := range ns {
		ns[i] = &Node{}
	}
	m.free = ns
	m.last = PRICE_SPOT
	return m
}

func (m *Mgr) String() string {
	s := fmt.Sprintf("{mgr nodes:")
	for _, n := range m.cur {
		s += fmt.Sprintf("{%v} ", n)
	}
	return s + "}"
}

func (m *Mgr) stats() {
	n := NTICK * NNODE
	fmt.Printf("Mgr: last %v revenue %v avg rev/tick %v util %.2f idle %dT nevict %dP nwasted %dT\n", m.last, m.revenue, Price(float64(m.revenue)/float64(m.nwork)), float64(m.nwork)/float64(n), m.nidle, m.nevict, m.nwasted)
}

func (m *Mgr) yield(n *Node) {
	if DEBUG {
		fmt.Printf("yield %v\n", n)
	}
	n.tenant = nil
	m.free = append(m.free, n)
	m.cur.remove(n)
}

func (m *Mgr) collectBids() Bids {
	bids := make([]*Bid, 0)
	for i, _ := range m.sim.tenants {
		if b := m.sim.tenants[i].bid(m.last); b != nil {
			// sort the bids in b
			sort.Slice(b.bids, func(i, j int) bool {
				return b.bids[i] > b.bids[j]
			})
			bids = append(bids, b)
		}
	}
	return bids
}

func (m *Mgr) checkAssignment(s string) {
	for _, n := range m.cur {
		if !n.tenant.isPresent(n) {
			fmt.Printf("node %v\n", n)
			fmt.Printf("m.cur %v\n", m.cur)
			fmt.Printf("m.tenant.nodes %v\n", n.tenant.nodes)
			panic(s)
		}
	}
}

func (m *Mgr) assignNodes() (Nodes, Price) {
	m.checkAssignment("before")
	bids := m.collectBids()
	// fmt.Printf("bids %v #free nodes %d\n", bids, len(m.free))
	new := make(Nodes, 0)
	m.avgbid = Price(0.0)
	m.high = Price(0.0)
	naccept := 0
	for {
		t, bid := bids.PopHighest(m.sim.rand)
		if t == nil {
			break
		}

		m.last = bid
		if bid > m.high {
			m.high = bid
		}
		m.avgbid += bid

		// fmt.Printf("assignNodes: %p bid highest %v\n", t, bid)
		if n := m.free.findFree(); n != nil {
			n.tenant = t
			n.price = bid
			//fmt.Printf("assignNodes: allocate %p to %p at %v\n", n, t, bid)
			t.grantNode(n)
			new = append(new, n)
		} else if n := m.cur.findVictim(t, bid); n != nil {
			if DEBUG {
				fmt.Printf("assignNodes: reallocate %v to %p at %v\n", n, t, bid)
			}
			m.nevict++
			n.tenant.evict(n)
			n.tenant = t
			n.price = bid
			n.tenant.grantNode(n)
		} else {
			// fmt.Printf("assignNodes: no nodes left\n")
			break
		}
		naccept++
	}
	price := m.last
	m.cur = append(m.cur, new...)
	// fmt.Printf("assignment %d nodes: %v\n", len(m.cur), m.cur)
	m.checkAssignment("after")

	// if idle nodes, lower price
	idle := uint64(NNODE - len(m.cur))
	m.nidle += idle
	if idle > 0 {
		m.last -= BID_INCREMENT
	}

	// avg bid for stats
	if naccept > 0 {
		m.avgbid = m.avgbid / Price(naccept)
	}

	return m.cur, price
}

//
// Run simulation
//

type Sim struct {
	time     uint64
	tenants  [NTENANT]Tenant
	rand     *rand.Rand
	mgr      *Mgr
	nproc    int    // total # procs started
	len      uint64 // sum of all procs len
	nprocq   uint64
	avgprice Price // avg price per tick
}

func mkSim(p Tpolicy) *Sim {
	sim := &Sim{}
	sim.rand = rand.New(rand.NewSource(time.Now().UnixNano()))

	sim.mgr = mkMgr(sim)
	for i := 0; i < NTENANT; i++ {
		t := &sim.tenants[i]
		t.procs = make([]*Proc, 0)
		t.sim = sim
		t.policy = p
		if i == 0 {
			t.poisson = &distuv.Poisson{Lambda: 10 * AVG_ARRIVAL_RATE}
		} else {
			t.poisson = &distuv.Poisson{Lambda: AVG_ARRIVAL_RATE}
		}
	}
	return sim
}

// At each tick, a tenants generates load in the form of procs that
// need to be run.
func (sim *Sim) genLoad() {
	for i, _ := range sim.tenants {
		p, l := sim.tenants[i].genProcs()
		sim.nproc += p
		sim.len += l
	}
}

func (sim *Sim) scheduleNodes() int {
	pq := 0
	for i, _ := range sim.tenants {
		pq += sim.tenants[i].scheduleNodes()
	}
	return pq
}

func (sim *Sim) printTenants(tick uint64, nn, pq int) {
	fmt.Printf("Tick %d nodes %d procq %d new price %v avgbid %v high %v", tick, nn, pq, sim.mgr.last, sim.mgr.avgbid, sim.mgr.high)
	for i, _ := range sim.tenants {
		t := &sim.tenants[i]
		if len(t.procs) > 0 || len(t.nodes) > 0 {
			fmt.Printf("\n%p: %v", t, t)
		}
	}
	fmt.Printf("\n")
}

func (sim *Sim) runProcs(ns Nodes, p Price) {
	sim.avgprice += p
	for _, n := range ns {
		if n.proc != nil {
			n.proc.nTick--
			n.proc.cost += p
			n.tenant.cost += p
			n.tenant.nwork++
			sim.mgr.nwork++
			sim.mgr.revenue += p * Price(len(ns))
			if n.proc.nTick == 0 {
				n.proc = nil
			}
		}
	}
}

func (sim *Sim) tick(tick uint64) {
	sim.genLoad()
	ns, p := sim.mgr.assignNodes()
	pq := sim.scheduleNodes()
	sim.nprocq += uint64(pq)

	if DEBUG {
		sim.printTenants(tick, len(ns), pq)
	}
	sim.runProcs(ns, p)
}

func funcName(i interface{}) string {
	return runtime.FuncForPC(reflect.ValueOf(i).Pointer()).Name()
}

func main() {
	policies := []Tpolicy{policyFixed, policyLast, policyBigMore}
	for i := 0; i < NTRIAL; i++ {
		for _, p := range policies {
			fmt.Printf("=== Policy %s\n", funcName(p))
			sim := mkSim(p)
			for ; sim.time < NTICK; sim.time++ {
				sim.tick(sim.time)
			}
			if DEBUG {
				for i, _ := range sim.tenants {
					sim.tenants[i].stats()
				}
			} else {
				sim.tenants[0].stats()
				sim.tenants[1].stats()
			}
			sim.mgr.stats()
			n := float64(NTICK)
			fmt.Printf("nproc %dP len %dT avg proclen %.2fT avg procq %.2fP/T avg price %v/T\n", sim.nproc, sim.len, float64(sim.len)/float64(sim.nproc), float64(sim.nprocq)/n, sim.avgprice/Price(n))
		}
	}
}
