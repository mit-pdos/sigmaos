package simmsched

import (
	"fmt"
	"math/rand"
	"time"
)

type Tmem int
type Ttick int

const (
	MAX_SERVICE_TIME = 10 // in ticks
	MAX_MEM          = 10
)

type TrealmId int
type Tftick float64

type Ttickmap map[TrealmId]Tftick
type Tprocmap map[TrealmId]int

func (f Tftick) String() string {
	return fmt.Sprintf("%.3fT", f)
}

type Proc struct {
	nTick Tftick
	nMem  Tmem
	realm TrealmId
}

func (p *Proc) String() string {
	return fmt.Sprintf("{nT %v nMem %d r %d}", p.nTick, p.nMem, p.realm)
}

func newProc(nTick Tftick, nMem Tmem, r TrealmId) *Proc {
	return &Proc{nTick, nMem, r}
}

type Queue struct {
	q []*Proc
}

func newQueue() *Queue {
	q := &Queue{q: make([]*Proc, 0)}
	return q
}

func (q *Queue) String() string {
	str := ""
	for _, p := range q.q {
		str += p.String()
	}
	return str
}

func (q *Queue) enq(p *Proc) {
	q.q = append(q.q, p)
}

func (q *Queue) find(n Tmem) *Proc {
	for i, p := range q.q {
		if p.nMem <= n {
			q.q = append(q.q[0:i], q.q[i+1:]...)
			return p
		}
	}
	return nil
}

func (q *Queue) zap(proc int) {
	q.q = append(q.q[0:proc], q.q[proc+1:]...)
}

func (q *Queue) run() Ttickmap {
	n := Tftick(float64(1.0 / float64(len(q.q))))
	ticks := make(Ttickmap)
	for n > 0 && len(q.q) > 0 {
		ps := make([]*Proc, 0)
		over := Tftick(0)
		for _, p := range q.q {
			u := n
			if p.nTick < n {
				u = p.nTick
				over += n - u
				p.nTick = 0
			} else {
				p.nTick -= n
			}
			if _, ok := ticks[p.realm]; ok {
				ticks[p.realm] += u
			} else {
				ticks[p.realm] = u
			}
			if p.nTick > 0 {
				ps = append(ps, p)
			}
		}
		q.q = ps
		if len(q.q) > 0 {
			n = Tftick(float64(over) / float64(len(q.q)))
			if n > 0.001 {
				fmt.Printf("another round of scheduling %v\n", n)
			} else {
				n = Tftick(0)
			}
		}
	}
	return ticks
}

func (q *Queue) mem() Tmem {
	m := Tmem(0)
	for _, p := range q.q {
		m += p.nMem
	}
	return m
}

func (q *Queue) qlen() int {
	return len(q.q)
}

type ProcQ struct {
	qs map[TrealmId]*Queue
}

func (pq *ProcQ) String() string {
	str := "[\n"
	for r, q := range pq.qs {
		str += fmt.Sprintf("  realm %d (%d)\n", r, q.qlen())
	}
	str += "  ]"
	return str
}

func (pq *ProcQ) addRealm(realm TrealmId) {
	pq.qs[realm] = newQueue()
}

func (pq *ProcQ) enq(p *Proc) {
	pq.qs[p.realm].enq(p)
}

// Starting from a random offset, sweep through realms finding a proc
// to run
func (pq *ProcQ) deq(n Tmem) *Proc {
	if len(pq.qs) == 0 {
		return nil
	}
	r := TrealmId(int(rand.Uint64() % uint64(len(pq.qs))))
	for i := 0; i < len(pq.qs); i++ {
		// XXX iterate through keys instead of assuming keys are consecutive
		if q, ok := pq.qs[r]; ok {
			if p := q.find(n); p != nil {
				return p
			}
		}
		r = TrealmId((int(r) + 1) % len(pq.qs))
	}
	return nil
}

func (pq *ProcQ) rqlen(r TrealmId) int {
	return pq.qs[r].qlen()
}

func (pq *ProcQ) qlen() int {
	l := 0
	for _, q := range pq.qs {
		l += q.qlen()
	}
	return l
}

type Schedd struct {
	totMem Tmem
	q      *Queue
	util   float64
	ticks  Ttickmap
	rutil  Ttickmap
}

func newSchedd() *Schedd {
	sd := &Schedd{
		totMem: MAX_MEM,
		q:      newQueue(),
		ticks:  make(Ttickmap),
		rutil:  make(Ttickmap),
	}
	return sd
}

func (sd *Schedd) String() string {
	return fmt.Sprintf("{totMem %d nMem %d q %v}", sd.totMem, sd.mem(), sd.q)
}

func (sd *Schedd) addRealm(realm TrealmId) {
	sd.ticks[realm] = Tftick(0)
	sd.rutil[realm] = Tftick(0)
}

func (sd *Schedd) mem() Tmem {
	return sd.q.mem()
}

func (sd *Schedd) run() {
	if len(sd.q.q) == 0 {
		return
	}
	for r, _ := range sd.rutil {
		sd.rutil[r] = Tftick(0)
	}
	sd.util += float64(1)
	ticks := sd.q.run()
	for r, t := range ticks {
		sd.ticks[r] += t
		sd.rutil[r] = t
	}
}

func (sd *Schedd) zap(r TrealmId) bool {
	proc := -1
	m := Tmem(0)
	for i, p := range sd.q.q {
		if p.realm == r && p.nMem > m {
			proc = i
			m = p.nMem
		}
	}
	if proc != -1 {
		sd.q.zap(proc)
		return true
	}
	return false
}

type Irealm interface {
	Id() TrealmId
	genLoad(rand *rand.Rand) []*Proc
}

type Realm struct {
	realm Irealm
}

func newRealm(realm Irealm) *Realm {
	r := &Realm{realm: realm}
	return r
}

type World struct {
	ntick   Ttick
	mscheds []*Schedd
	procqs  []*ProcQ
	realms  map[TrealmId]*Realm
	rand    *rand.Rand
	nproc   Tprocmap
	maxq    int
	avgq    float64
}

func newWorld(nProcQ, nSchedd int) *World {
	w := &World{}
	w.mscheds = make([]*Schedd, nSchedd)
	w.procqs = make([]*ProcQ, nProcQ)
	for i := 0; i < len(w.mscheds); i++ {
		w.mscheds[i] = newSchedd()
	}
	for i := 0; i < len(w.procqs); i++ {
		w.procqs[i] = &ProcQ{qs: make(map[TrealmId]*Queue)}
	}
	w.realms = make(map[TrealmId]*Realm)
	w.rand = rand.New(rand.NewSource(time.Now().UnixNano()))
	w.nproc = make(Tprocmap)
	return w
}

func (w *World) String() string {
	str := fmt.Sprintf("%d nrealm %d nproc %v ntick/r %v maxq %d avgq %.1f util %.1f%%\n mscheds:", w.ntick, len(w.realms), w.nproc, w.fairness(), w.maxq, w.avgq/float64(w.ntick), w.util())
	str += "[\n"
	for _, sd := range w.mscheds {
		str += "  " + sd.String() + ",\n"
	}
	str += "  ]\n procQs:"
	for _, pq := range w.procqs {
		str += pq.String()
	}
	return str
}

func (w *World) addRealm(realm Irealm) {
	id := realm.Id()
	w.realms[id] = newRealm(realm)
	w.nproc[id] = 0
	for _, sd := range w.mscheds {
		sd.addRealm(id)
	}
	for _, pq := range w.procqs {
		pq.addRealm(id)
	}
}

func (w *World) fairness() Ttickmap {
	ntick := make(Ttickmap)
	for _, sd := range w.mscheds {
		for r, n := range sd.ticks {
			if _, ok := ntick[r]; ok {
				ntick[r] += n
			} else {
				ntick[r] = n
			}
		}
	}
	return ntick
}

func (w *World) util() float64 {
	u := float64(0)
	for _, sd := range w.mscheds {
		u += sd.util
	}
	return (u / float64(w.ntick)) * float64(100)
}

func (w *World) genLoad() {
	for _, r := range w.realms {
		procs := r.realm.genLoad(w.rand)
		for _, p := range procs {
			q := int(rand.Uint64() % uint64(len(w.procqs)))
			w.nproc[p.realm] += 1
			w.procqs[q].enq(p)
		}
	}
}

// Try go get a proc from one random ProcQ server
func (w *World) getProc(n Tmem) *Proc {
	q := int(rand.Uint64() % uint64(len(w.procqs)))
	for i := 0; i < 1; i++ {
		// XXX iterate through keys instead of assuming keys/realms are consecutive
		if p := w.procqs[q].deq(n); p != nil {
			return p
		}
		q = (q + 1) % len(w.procqs)
	}
	return nil
}

func (w *World) getProcs() {
	capacityAvailable := true
	i := 0
	for capacityAvailable {
		c := false
		for _, sd := range w.mscheds {
			m := sd.mem()
			if m < sd.totMem {
				if p := w.getProc(sd.totMem - m); p != nil {
					c = true
					sd.q.enq(p)
				}
			}
		}
		capacityAvailable = c
		i += 1
	}
	for _, sd := range w.mscheds {
		m := sd.mem()
		if m < sd.totMem {
			fmt.Printf("WARNING CAPACITY %v\n", sd)
		}
	}
}

func (w *World) compute() {
	for _, sd := range w.mscheds {
		sd.run()
	}
}

func (w *World) zap(r TrealmId) {
	fmt.Printf("zap a proc from realm %v at %v\n", r, w.ntick)
	for _, sd := range w.mscheds {
		if sd.zap(r) {
			return
		}
	}
}

func (w *World) utilPerRealm() Ttickmap {
	rutil := make(Ttickmap)
	for _, sd := range w.mscheds {
		for r, t := range sd.rutil {
			if _, ok := rutil[r]; ok {
				rutil[r] += t
			} else {
				rutil[r] = t
			}
		}
	}
	return rutil
}

func (w *World) hasWork(lr TrealmId) bool {
	for _, pq := range w.procqs {
		if pq.rqlen(lr) > 0 {
			return true
		}
	}
	return false
}

func (w *World) utilRange(rutil Ttickmap) (Tftick, TrealmId, Tftick, TrealmId) {
	h := Tftick(0)
	hr := TrealmId(0)
	l := Tftick(len(w.mscheds))
	lr := TrealmId(0)
	for r, u := range rutil {
		if u > h {
			h = u
			hr = r
		}
		if u < l {
			l = u
			lr = r
		}
	}
	return h, hr, l, lr
}

func (w *World) zapper() {
	rutil := w.utilPerRealm()
	avg := Tftick(len(w.mscheds) / len(rutil))
	h, hr, l, lr := w.utilRange(rutil)
	// fmt.Printf("rutil %v avg %v h %v hr %v l %v lr %v\n", rutil, avg, h, hr, l, lr)
	if h-l > avg*1.25 {
		if w.hasWork(lr) {
			w.zap(hr)
		}
	}
}

func (w *World) qstat() {
	qlen := 0
	for _, pq := range w.procqs {
		qlen += pq.qlen()
	}
	w.avgq += float64(qlen)
	if qlen > w.maxq {
		w.maxq = qlen
	}
}

func (w *World) Tick() {
	w.ntick += 1
	w.genLoad()
	fmt.Printf("after gen %v\n", w)
	w.getProcs()
	fmt.Printf("after getprocs %v\n", w)
	w.compute()
	w.zapper()
	w.qstat()
	fmt.Printf("after compute %v\n", w)
}

func zipf(r *rand.Rand) uint64 {
	z := rand.NewZipf(r, 2.0, 1.0, MAX_SERVICE_TIME-1)
	return z.Uint64() + 1
}
