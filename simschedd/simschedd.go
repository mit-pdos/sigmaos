package simschedd

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

func (f Tftick) String() string {
	return fmt.Sprintf("%.1fT", f)
}

type Proc struct {
	nTick Tftick
	nMem  Tmem
	realm TrealmId
}

func (p *Proc) String() string {
	return fmt.Sprintf("{nT %v nMem %d r %d}", p.nTick, p.nMem, p.realm)
}

func newProc(nTick Ttick, nMem Tmem, r TrealmId) *Proc {
	return &Proc{Tftick(nTick), nMem, r}
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

func (q *Queue) run(n Tftick) Ttickmap {
	ticks := make(Ttickmap)
	ps := make([]*Proc, 0)
	for _, p := range q.q {
		p.nTick -= n
		if _, ok := ticks[p.realm]; ok {
			ticks[p.realm] += n
		} else {
			ticks[p.realm] = n
		}
		if p.nTick > 0 {
			ps = append(ps, p)
		}
	}
	q.q = ps
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
		str += fmt.Sprintf("  realm %d (%d): %v,\n", r, q.qlen(), q)
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

func (pq *ProcQ) deq(n Tmem) *Proc {
	if len(pq.qs) == 0 {
		return nil
	}
	r := TrealmId(int(rand.Uint64() % uint64(len(pq.qs))))
	if q, ok := pq.qs[r]; ok {
		return q.find(n)
	}
	return nil
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
	ticks  map[TrealmId]Tftick
}

func newSchedd() *Schedd {
	sd := &Schedd{totMem: MAX_MEM, q: newQueue(), ticks: make(map[TrealmId]Tftick)}
	return sd
}

func (sd *Schedd) String() string {
	return fmt.Sprintf("{totMem %d nMem %d q %v}", sd.totMem, sd.mem(), sd.q)
}

func (sd *Schedd) addRealm(realm TrealmId) {
	sd.ticks[realm] = Tftick(0)
}

func (sd *Schedd) mem() Tmem {
	return sd.q.mem()
}

func (sd *Schedd) run() {
	if len(sd.q.q) == 0 {
		return
	}
	n := Tftick(float64(1.0 / float64(len(sd.q.q))))
	sd.util += float64(1)
	ticks := sd.q.run(n)
	for r, t := range ticks {
		sd.ticks[r] += t
	}
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
	schedds []*Schedd
	procqs  []*ProcQ
	realms  map[TrealmId]*Realm
	rand    *rand.Rand
	nproc   int
	nwork   Tftick
	maxq    int
	avgq    float64
}

func newWorld(nProcQ, nSchedd int) *World {
	w := &World{}
	w.schedds = make([]*Schedd, nSchedd)
	w.procqs = make([]*ProcQ, nProcQ)
	for i := 0; i < len(w.schedds); i++ {
		w.schedds[i] = newSchedd()
	}
	for i := 0; i < len(w.procqs); i++ {
		w.procqs[i] = &ProcQ{qs: make(map[TrealmId]*Queue)}
	}
	w.realms = make(map[TrealmId]*Realm)
	w.rand = rand.New(rand.NewSource(time.Now().UnixNano()))
	return w
}

func (w *World) String() string {
	str := fmt.Sprintf("%d nrealm %d nproc %d (ntick %v) nwork %v maxq %d avgq %.1f util %.1f%%\n schedds:", w.ntick, len(w.realms), w.nproc, w.fairness(), w.nwork, w.maxq, w.avgq/float64(w.ntick), w.util())
	str += "[\n"
	for _, sd := range w.schedds {
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
	for _, sd := range w.schedds {
		sd.addRealm(id)
	}
	for _, pq := range w.procqs {
		pq.addRealm(id)
	}
}

func (w *World) fairness() []Tftick {
	ntick := make([]Tftick, len(w.realms))
	for _, sd := range w.schedds {
		for i, n := range sd.ticks {
			ntick[i] += n
		}
	}
	return ntick
}

func (w *World) util() float64 {
	u := float64(0)
	for _, sd := range w.schedds {
		u += sd.util
	}
	return (u / float64(w.ntick)) * float64(100)
}

func (w *World) genLoad() {
	for _, r := range w.realms {
		procs := r.realm.genLoad(w.rand)
		for _, p := range procs {
			q := int(rand.Uint64() % uint64(len(w.procqs)))
			w.nproc += 1
			w.nwork += p.nTick
			w.procqs[q].enq(p)
		}
	}
}

func (w *World) getProc(n Tmem) *Proc {
	for i := 0; i < 1; i++ {
		q := int(rand.Uint64() % uint64(len(w.procqs)))
		if p := w.procqs[q].deq(n); p != nil {
			return p
		}
	}
	return nil
}

func (w *World) getProcs() {
	for _, sd := range w.schedds {
		m := sd.mem()
		if m < sd.totMem {
			if p := w.getProc(sd.totMem - m); p != nil {
				sd.q.enq(p)
			}
		}
	}
}

func (w *World) compute() {
	for _, sd := range w.schedds {
		sd.run()
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
	w.compute()
	w.qstat()
	fmt.Printf("after compute %v\n", w)
}

func zipf(r *rand.Rand) uint64 {
	z := rand.NewZipf(r, 2.0, 1.0, MAX_SERVICE_TIME-1)
	return z.Uint64() + 1
}
