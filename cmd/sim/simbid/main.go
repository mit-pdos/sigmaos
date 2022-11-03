package main

import (
	"fmt"
	"math"
	"math/rand"
	"reflect"
	"runtime"
	"sort"
	"time"

	"gonum.org/v1/gonum/stat/distuv"
)

const (
	DEBUG  = true
	NTRIAL = 1

	AVG_ARRIVAL_RATE float64 = 0.1 // per tick
	MAX_SERVICE_TIME         = 10  // in ticks

	PRICE_ONDEMAND Price = 0.00000001155555555555 // per ms for 1h on AWS
	PRICE_SPOT     Price = 0.00000000347222222222 // per ms
	BID_INCREMENT  Price = 0.000000000001
	MAX_BID        Price = 3 * PRICE_SPOT
)

//
// Worlds being simulated
//

type World struct {
	nNode           int
	nTenant         int
	nodesPerMachine int
	lambdas         []float64
	nTick           Tick
	policy          Tpolicy
	tick            Tick
}

func mkWorld(n, t, npm int, ls []float64, nt Tick, p Tpolicy) *World {
	w := &World{}
	w.nNode = n
	w.nTenant = t
	w.nodesPerMachine = npm
	w.lambdas = ls
	w.nTick = nt
	w.policy = p
	return w
}

var world *World

func zipf(r *rand.Rand) uint64 {
	z := rand.NewZipf(r, 2.0, 1.0, MAX_SERVICE_TIME-1)
	return z.Uint64() + 1
}

func uniform(r *rand.Rand) uint64 {
	return (rand.Uint64() % MAX_SERVICE_TIME) + 1
}

type Tpolicy func(*Tenant, Price) *Bid
type Tmid int

// Tick
type Tick uint64

func (t Tick) String() string {
	return fmt.Sprintf("%dT", t)
}

//
// Fractional tick
//

type FTick float64

func (f FTick) String() string {
	return fmt.Sprintf("%.1fT", f)
}

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
	if len(bs) == 0 {
		return nil
	}
	return &Bid{t, bs}
}

type Bids []*Bid

func (bs *Bids) PopHighest(rand *rand.Rand) (*Tenant, Price) {
	bid := Price(0.0)

	if len(*bs) == 0 {
		return nil, bid
	}

	// fmt.Printf("bids %v\n", *bs)

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
	nLength Tick  //
	nTick   FTick // # fractional ticks remaining
	time    Tick  // # ticks on a node
	cost    Price // cost for this proc
	// compute intensity: 1.0 computes only, while 0.4 is doing i/o
	// for 0.6T, which overlaps with the computing happening in a
	// tick.
	computeT FTick
}

func (p *Proc) String() string {
	return fmt.Sprintf("{n %v c %v t %v l %v}", p.nTick, p.computeT, p.time, p.nLength)
}

func mkProc(rand *rand.Rand) *Proc {
	p := &Proc{}
	// p.nTick = zipf(rand)
	t := Tick(uniform(rand))
	p.nTick = FTick(t)
	p.nLength = t
	p.computeT = 0.5
	return p
}

type Procs []*Proc

// Run procs ps of a node until node used 1 tick of cpu compute or ps
// runs out of procs.  The last selected proc may run only for a
// fraction of tick.  Return how much we used of the 1 tick, and for
// procs that finished how much delay was incurred (because the nodes
// was overloaded).
func (ps *Procs) run(c Price) (FTick, Tick) {
	work := FTick(0.0)
	last := FTick(0.0)
	delay := Tick(0.0)

	// compute number of procs we can run until we hit 1 tick
	n := 0
	for _, p := range *ps {
		n += 1
		w := p.computeT
		if p.nTick < 1 { // only fraction of tick left?
			w = p.computeT * p.nTick
		}
		tickLeft := FTick(1.0 - work)
		if w < tickLeft {
			last = 1
			work += FTick(w)
		} else {
			last = tickLeft
			work += FTick(last)
			break
		}
	}
	// fmt.Printf("ps %v work %v last %v\n", *ps, work, last)
	if work > FTick(1.0) {
		panic("run: work")
	}

	for _, p := range *ps {
		p.time += 1
	}

	// run the first n
	qs := (*ps)[0:n]
	*ps = (*ps)[n:]
	for i, p := range qs {
		if i < len(qs)-1 || last == 1 {
			// If this wasn't the last proc it definitely ran for a
			// full tick, or as much of a tick as it had left, which
			// may be < 1.  If it was the last proc but runs for a
			// full tick, then also subtract 1 tick.
			p.nTick--
			if p.nTick < 0 {
				p.nTick = 0
			}
		} else {
			// If this was the last proc and it ran for a partial
			// "last" tick.  In this partial tick, p can do
			// last/p.computeT of its tick, some computing and some
			// sleeping.
			p.nTick -= last / p.computeT
		}

		// charge every proc equally, even though last proc may not
		// get to run for p.computeT.
		p.cost += c / Price(len(qs))

		// Ensure no proc ever runs for more ticks than its total length.
		if p.nTick < 0 {
			fmt.Printf("%v/%vth proc %v\n", i, len(qs), p)
			panic("Negative nTick")
		}

		if p.nTick == 0 { // p is done
			delay += p.time - p.nLength
		} else {
			// not done; put it at the end of procq so that procs run
			// round robin
			*ps = append(*ps, p)
		}
	}

	return work, delay
}

func (ps *Procs) wasted() (FTick, Price) {
	w := FTick(0)
	c := Price(0.0)
	for _, p := range *ps {
		w += FTick(p.nLength) - p.nTick // wasted ticks
		if w > 0 {
			c += p.cost
		}
	}
	return w, c
}

type Machine struct {
	mid     Tmid
	nodes   Nodes
	ntenant int
}

func (m *Machine) String() string {
	return fmt.Sprintf("{%d %d}", m.ntenant, len(m.nodes))
}

type Machines map[Tmid]*Machine

// Returns the m's (and their nodes) in ms that are also present in
// ms1 (which may have fewer nodes)
func (ms Machines) intersect(ms1 Machines) Machines {
	r := make(Machines)
	for k, m := range ms {
		if _, ok := ms1[k]; ok {
			r[k] = m
		}
	}
	return r
}

// Among ms find the machine most-heavily used
func (ms Machines) mostUsed() *Machine {
	var most *Machine
	high := 0
	for _, m := range ms {
		if m.ntenant > high && m.ntenant < world.nodesPerMachine {
			most = m
			high = m.ntenant
		}
	}
	return most
}

// Among ms find the machine least-heavily used
func (ms Machines) leastUsed() *Machine {
	var least *Machine
	low := world.nTenant
	for _, m := range ms {
		if m.ntenant < low {
			least = m
			low = m.ntenant
		}
	}
	return least
}

// Find node on machine mid with the least amount of work done in last
// tick
func (ms Machines) findNodeOnMachine(mid Tmid) *Node {
	work := FTick(1.1)
	var r *Node
	for _, m := range ms {
		for _, n := range m.nodes {
			if n.mid == mid && n.work < work {
				r = n
				work = n.work
			}
		}
	}
	return r
}

//
// Computing nodes that the manager allocates to tenants.  A node
// time-shares the procs assigned to it.
//

type Node struct {
	procs  Procs
	price  Price // the price for a tick
	work   FTick // how much of the last tick was used to run procs
	tenant *Tenant
	mid    Tmid
}

func (n *Node) String() string {
	return fmt.Sprintf("{%p: proc %v price %v l %v t %p m %d}", n, n.procs, n.price, n.work, n.tenant, n.mid)
}

// XXX takes only 1 proc, which may leave the node idle for most of the tick
func (n *Node) takeProcs(ps Procs) Procs {
	if n.work >= FTick(1.0) {
		return ps
	}
	n.procs = append(n.procs, ps[0])
	ps = ps[1:]
	return ps
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

// Compute a "machine" view of ns. That is, return the machines used
// by ns, with for each machine the nodes are part of that machine.
func (ns *Nodes) machines() Machines {
	ms := make(map[Tmid]*Machine)
	for _, n := range *ns {
		if _, ok := ms[n.mid]; !ok {
			m := &Machine{}
			m.nodes = make(Nodes, 0)
			ms[n.mid] = m
			m.mid = n.mid
		}
		m := ms[n.mid]
		m.nodes = append(m.nodes, n)
		if n.tenant != nil {
			m.ntenant += 1
		}
		if m.ntenant > world.nodesPerMachine {
			fmt.Printf("nodes %v\n", ns)
			panic("machines")
		}
	}
	return ms
}

func (ns *Nodes) findFree(tms Machines) *Node {
	if len(*ns) == 0 {
		return nil
	}
	fms := ns.machines()
	//ms1 := tms.intersect(fms)
	//m := ms1.mostUsed()
	//if m == nil {
	m := fms.leastUsed()
	//}
	for _, n := range m.nodes {
		if n.tenant == nil {
			ns.remove(n)
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

func (ns *Nodes) check() {
	m := make(map[*Node]bool)
	for _, n := range *ns {
		_, ok := m[n]
		if !ok {
			m[n] = true
		} else {
			fmt.Printf("double %v\n", ns)
			panic("check")
		}
	}
}

func (ns Nodes) nproc() int {
	np := 0
	for _, n := range ns {
		np += len(n.procs)
	}
	return np
}

// Schedule procs in ps on the nodes in ns
func (ns Nodes) schedule(ps Procs) Procs {
	for _, n := range ns {
		if len(ps) == 0 { // no procs left to schedule?
			break
		}
		ps = n.takeProcs(ps)
	}
	return ps
}

//
// Tenants run procs on the nodes allocated to them by the mgr. If a
// tenant has more procs to run than available nodes, tenant bids for
// more nodes.
//

type Tenant struct {
	poisson  *distuv.Poisson
	procs    []*Proc
	nodes    Nodes
	nbid     int
	ngrant   int
	sim      *Sim
	nproc    int  // sum of # procs
	ntick    Tick // sum of # ticks
	maxnode  int
	nwork    FTick  // sum of # tick fractions running a proc
	cost     Price  // cost for nwork ticks
	nwait    Tick   // sum of # ticks waiting to be run
	ndelay   Tick   // sum of # extra ticks that proc was on node
	nmigrate uint64 // # procs migrated
	nevict   uint64 // # evicted procs
	nwasted  FTick  // sum # fticks wasted because of eviction
	sunkCost Price  // the cost of the wasted ticks
}

func (t *Tenant) String() string {
	s := fmt.Sprintf("{nproc %d ntick %d procq (%d): [", t.nproc, t.ntick, len(t.procs))
	for _, p := range t.procs {
		s += fmt.Sprintf("{%v} ", p)
	}
	s += fmt.Sprintf("] nodes (%d): [", len(t.nodes))
	for _, n := range t.nodes {
		s += fmt.Sprintf("%v ", n)
	}
	ms := t.nodes.machines()
	s += fmt.Sprintf("ms (%d) %v", len(ms), ms)
	return s + "]}"
}

// New procs "arrive" based on Poisson distribution. Schedule queued
// procs on the available nodes, and release nodes we don't use.
func (t *Tenant) genProcs() (int, Tick) {
	nproc := int(t.poisson.Rand())
	len := Tick(0)
	for i := 0; i < nproc; i++ {
		p := mkProc(t.sim.rand)
		len += p.nLength
		t.procs = append(t.procs, p)
	}
	t.nproc += nproc
	t.procs = t.nodes.schedule(t.procs)
	t.yieldIdle()
	return nproc, len
}

// XXX give priorities to procs and use that in bid
func policyBidMore(t *Tenant, last Price) *Bid {
	nprocs := t.nodes.nproc()
	nnodes := len(t.nodes)
	nproc_node := float64(0)
	nbid := 0
	if nnodes > 0 {
		nproc_node = float64(nprocs) / float64(nnodes)
		nbid = int(math.Round(float64(len(t.procs)) / nproc_node))
	} else if len(t.procs) > 0 {
		nbid = 1
	}
	//fmt.Printf("%p: procq %d nprocs %d nnodes %d %.2f nbid %d\n", t, len(t.procs), nprocs, nnodes, nproc_node, nbid)
	bids := make([]Price, 0)
	for i := 0; i < nbid; i++ {
		bid := last + BID_INCREMENT*Price(len(t.procs))
		//bid := last + BID_INCREMENT
		//bid := last
		bids = append(bids, bid)
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
	return world.policy(t, last)
}

// mgr grants a node
func (t *Tenant) grantNode(n *Node) {
	t.ngrant++
	t.nodes = append(t.nodes, n)
	t.nodes.check()

	if len(t.nodes) > 6 {
		panic("xxx")
	}

	t.nodes.machines()
}

// After bidding, we may have received ngrant new nodes; schedule
// procs on them.
func (t *Tenant) schedule() int {
	if DEBUG {
		if t.nbid > 0 && t.ngrant < t.nbid {
			fmt.Printf("%v %p: asked %d and received %d\n", world.tick, t, t.nbid, t.ngrant)
		}
	}

	t.procs = t.nodes[len(t.nodes)-t.ngrant:].schedule(t.procs)

	t.ntick += Tick(len(t.nodes))
	if len(t.nodes) > t.maxnode {
		t.maxnode = len(t.nodes)
	}
	t.nwait += Tick(uint64(len(t.procs)))
	return len(t.procs)
}

// Yield idle nodes, except if tenant "reserved" the node
func (t *Tenant) yieldIdle() {
	for i := 0; i < len(t.nodes); i++ {
		n := t.nodes[i]
		if len(n.procs) == 0 && n.price != PRICE_ONDEMAND {
			t.nodes = append(t.nodes[0:i], t.nodes[i+1:]...)
			i--
			t.sim.mgr.yield(n)
		}
	}
}

// Manager is taking away node n
func (t *Tenant) evict(n *Node) (uint64, uint64) {
	if t.nodes.remove(n) == nil {
		fmt.Printf("%p: n not found %v\n", t, n)
		panic("evict")
	}
	ms := t.nodes.machines()
	e := uint64(0)
	m := uint64(0)
	if n1 := ms.findNodeOnMachine(n.mid); n1 != nil {
		if DEBUG {
			fmt.Printf("%v: Migrate %v to %v\n", world.tick, n, n1)
		}
		m += uint64(len(n.procs))
		n1.procs = append(n1.procs, n.procs...)
	} else {
		if DEBUG {
			fmt.Printf("%v: Evict %v\n", world.tick, n)
		}
		w, c := n.procs.wasted()
		e += uint64(len(n.procs))
		t.nwasted += w
		t.sim.mgr.nwasted += w
		t.sunkCost += c
	}
	t.nevict += e
	t.nmigrate += m
	n.procs = make(Procs, 0)
	return e, m
}

func (t *Tenant) isPresent(n *Node) bool {
	return t.nodes.isPresent(n)
}

func (t *Tenant) stats() {
	n := float64(world.nTick)
	fmt.Printf("%p: p %dP l %v P/T %.2f T/P maxN %d work %v util %.2f nwait %v ndelay %v #migr %dP #evict %dP (waste %v) charge %v sunk %v tick %v\n", t, t.nproc, float64(t.nproc)/n, float64(t.ntick)/float64(t.nproc), t.maxnode, t.nwork, float64(t.nwork)/float64(t.ntick), t.nwait, t.ndelay, t.nmigrate, t.nevict, t.nwasted, t.cost, t.sunkCost, t.cost/Price(t.nwork))
}

//
// Manager assigns nodes to tenants
//

type Mgr struct {
	sim      *Sim
	free     Nodes
	cur      Nodes
	index    int
	revenue  Price
	nwork    FTick
	nidle    uint64
	nevict   uint64
	nmigrate uint64
	nwasted  FTick
	last     Price // lowest bid accepted in last tick
	avgbid   Price // avg bid in last tick
	high     Price // highest bid in last tick
}

func mkMgr(sim *Sim) *Mgr {
	m := &Mgr{}
	m.sim = sim
	ns := make(Nodes, world.nNode, world.nNode)
	for i, _ := range ns {
		ns[i] = &Node{}
		ns[i].mid = Tmid(i / world.nodesPerMachine)
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
	n := world.nTick * Tick(world.nNode)
	fmt.Printf("Mgr: last %v revenue %v avg rev/tick %v util %.2f idle %v nmigrate %dP nevict %dP nwasted %v\n", m.last, m.revenue, Price(float64(m.revenue)/float64(m.nwork)), float64(m.nwork)/float64(n), m.nidle, m.nmigrate, m.nevict, m.nwasted)
}

func (m *Mgr) yield(n *Node) {
	if DEBUG {
		fmt.Printf("%v: yield %v\n", world.tick, n)
	}
	n.work = FTick(0.0)
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

// Allocate n nodes at PRICE_ONDEMAND to tenant t
func (m *Mgr) allocNode(t *Tenant, n int) {
	for i := 0; i < n; i++ {
		ms := t.nodes.machines()
		if n := m.free.findFree(ms); n != nil {
			n.tenant = t
			n.price = PRICE_ONDEMAND
			t.grantNode(n)
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

		ms := t.nodes.machines()

		m.last = bid
		if bid > m.high {
			m.high = bid
		}
		m.avgbid += bid

		// fmt.Printf("assignNodes: %p bid highest %v\n", t, bid)
		if n := m.free.findFree(ms); n != nil {
			n.tenant = t
			n.price = bid
			if DEBUG {
				fmt.Printf("assignNodes: allocate %p to %p at %v\n", n, t, bid)
			}
			t.grantNode(n)
			new = append(new, n)
		} else if n := m.cur.findVictim(t, bid); n != nil {
			if DEBUG {
				fmt.Printf("%v: assignNodes: reallocate %v to %p at %v\n", world.tick, n, t, bid)
			}
			ev, mi := n.tenant.evict(n)
			n.tenant = t
			n.price = bid
			m.nevict += ev
			m.nmigrate += mi
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
	idle := uint64(world.nNode - len(m.cur))
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
	tenants  []Tenant
	rand     *rand.Rand
	mgr      *Mgr
	nproc    int  // total # procs started
	len      Tick // sum of all procs len
	nprocq   uint64
	avgprice Price // avg price per tick
}

func mkSim() *Sim {
	sim := &Sim{}
	sim.rand = rand.New(rand.NewSource(time.Now().UnixNano()))
	sim.mgr = mkMgr(sim)
	sim.tenants = make([]Tenant, world.nTenant, world.nTenant)
	for i := 0; i < world.nTenant; i++ {
		t := &sim.tenants[i]
		t.procs = make([]*Proc, 0)
		t.sim = sim
		t.poisson = &distuv.Poisson{Lambda: world.lambdas[i]}
		if i == 0 {
			// Allocate one high-priced node to sustain the expected
			// load of 1.
			sim.mgr.allocNode(t, 1)
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

func (sim *Sim) schedule() int {
	pq := 0
	for i, _ := range sim.tenants {
		pq += sim.tenants[i].schedule()
	}
	return pq
}

func (sim *Sim) printTenants(nn, pq int) {
	fmt.Printf("Tick %d nodes %d procq %d nwork %v new price %v avgbid %v high %v", world.tick, nn, pq, sim.mgr.nwork, sim.mgr.last, sim.mgr.avgbid, sim.mgr.high)
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
		w, d := n.procs.run(p)
		n.work = w
		n.tenant.ndelay += d
		n.tenant.cost += p
		n.tenant.nwork += w
		sim.mgr.nwork += w
	}
	sim.mgr.revenue += p * Price(len(ns))
}

func (sim *Sim) tick() {
	sim.genLoad()
	ns, p := sim.mgr.assignNodes()
	pq := sim.schedule()
	sim.nprocq += uint64(pq)

	if DEBUG {
		sim.printTenants(len(ns), pq)
	}
	sim.runProcs(ns, p)
}

func funcName(i interface{}) string {
	return runtime.FuncForPC(reflect.ValueOf(i).Pointer()).Name()
}

func runSim() {
	fmt.Printf("=== Policy %s (n/m %d)\n", funcName(world.policy), world.nodesPerMachine)

	sim := mkSim()
	for world.tick = 0; world.tick < world.nTick; world.tick++ {
		sim.tick()
	}
	if DEBUG {
		for i, _ := range sim.tenants {
			sim.tenants[i].stats()
		}
	} else {
		sim.tenants[0].stats()
		if world.nTenant >= 2 {
			sim.tenants[1].stats()
		}
		if world.nTenant >= 3 {
			sim.tenants[2].stats()
		}
	}
	sim.mgr.stats()
	n := float64(world.nTick)
	fmt.Printf("nproc %dP len %v avg proclen %.2fT avg procq %.2fP/T avg price %v/T\n", sim.nproc, sim.len, float64(sim.len)/float64(sim.nproc), float64(sim.nprocq)/n, sim.avgprice/Price(n))
}

func main() {
	// policies := []Tpolicy{policyFixed, policyLast, policyBidMore}
	policies := []Tpolicy{policyBidMore}
	//policies := []Tpolicy{policyFixed}

	nNode := 50
	nTenant := 2
	nTick := Tick(100)

	ls := make([]float64, nTenant, nTenant)
	ls[0] = 10 * AVG_ARRIVAL_RATE
	for i := 1; i < nTenant; i++ {
		ls[i] = AVG_ARRIVAL_RATE
	}

	//npm := []int{1, 5, nNode}
	npm := []int{1}

	for i := 0; i < NTRIAL; i++ {
		for _, p := range policies {
			for _, n := range npm {
				world = mkWorld(nNode, nTenant, n, ls, nTick, p)
				runSim()
			}
		}
	}
}
