package simschedd

import (
	"fmt"
	"math/rand"
	"time"

	"gonum.org/v1/gonum/stat/distuv"
)

type Tmem int
type Ttick int

const (
	MAX_SERVICE_TIME         = 10 // in ticks
	MAX_MEM                  = 10
	AVG_ARRIVAL_RATE float64 = 0.2 // per tick
)

type Trealm int
type Tftick float64

func (f Tftick) String() string {
	return fmt.Sprintf("%.1fT", f)
}

type Proc struct {
	nTick Tftick
	nMem  Tmem
	realm Trealm
}

func (p *Proc) String() string {
	return fmt.Sprintf("{nT %v nMem %d r %d}", p.nTick, p.nMem, p.realm)
}

func newProc(nTick Ttick, nMem Tmem, r Trealm) *Proc {
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

func (q *Queue) run(n Tftick) []*Proc {
	ps := make([]*Proc, 0)
	done := make([]*Proc, 0)
	for _, p := range q.q {
		p.nTick -= n
		if p.nTick > 0 {
			ps = append(ps, p)
		} else {
			done = append(done, p)
		}
	}
	q.q = ps
	return done
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
	qs map[Trealm]*Queue
}

func (pq *ProcQ) String() string {
	str := "[\n"
	for r, q := range pq.qs {
		str += fmt.Sprintf("  realm %d (%d): %v,\n", r, q.qlen(), q)
	}
	str += "  ]"
	return str
}

func (pq *ProcQ) enq(p *Proc) {
	q, ok := pq.qs[p.realm]
	if !ok {
		q = newQueue()
		pq.qs[p.realm] = q
	}
	q.enq(p)
	return
}

func (pq *ProcQ) deq(n Tmem) *Proc {
	if len(pq.qs) == 0 {
		return nil
	}
	r := Trealm(int(rand.Uint64() % uint64(len(pq.qs))))
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
	nproc  map[Trealm]int
}

func newSchedd(nrealm int) *Schedd {
	sd := &Schedd{totMem: MAX_MEM, q: newQueue(), nproc: make(map[Trealm]int)}
	for i := 0; i < nrealm; i++ {
		sd.nproc[Trealm(i)] = 0
	}
	return sd
}

func (sd *Schedd) String() string {
	return fmt.Sprintf("{totMem %d nMem %d q %v}", sd.totMem, sd.mem(), sd.q)
}

func (sd *Schedd) mem() Tmem {
	return sd.q.mem()
}

func (sd *Schedd) run() {
	if len(sd.q.q) == 0 {
		return
	}
	n := float64(1.0 / float64(len(sd.q.q)))
	sd.util += float64(1)
	ps := sd.q.run(Tftick(n))
	for _, p := range ps {
		sd.nproc[p.realm] += 1
	}
}

type Realm struct {
	realm   Trealm
	poisson *distuv.Poisson
}

func newRealm(lambda float64, realm Trealm) *Realm {
	r := &Realm{realm: realm, poisson: &distuv.Poisson{Lambda: lambda}}
	return r
}

func (r *Realm) genLoad(rand *rand.Rand) []*Proc {
	nproc := int(r.poisson.Rand())
	procs := make([]*Proc, nproc)
	for i := 0; i < nproc; i++ {
		t := Ttick(uniform(rand))
		m := Tmem(uniform(rand))
		procs[i] = newProc(t, m, r.realm)
	}
	return procs
}

type World struct {
	ntick   Ttick
	schedds []*Schedd
	procqs  []*ProcQ
	realms  []*Realm
	rand    *rand.Rand
	nproc   int
	nwork   Tftick
	maxq    int
	avgq    float64
	lambda  float64
}

func newWorld(nProcQ, nSchedd, nRealm int) *World {
	w := &World{}
	w.schedds = make([]*Schedd, nSchedd)
	w.procqs = make([]*ProcQ, nProcQ)
	for i := 0; i < len(w.schedds); i++ {
		w.schedds[i] = newSchedd(nRealm)
	}
	for i := 0; i < len(w.procqs); i++ {
		w.procqs[i] = &ProcQ{qs: make(map[Trealm]*Queue)}
	}
	w.realms = make([]*Realm, nRealm)
	w.lambda = AVG_ARRIVAL_RATE * (float64(nSchedd) / float64(nRealm))
	for i := 0; i < len(w.realms); i++ {
		w.realms[i] = newRealm(w.lambda, Trealm(i))
	}
	w.rand = rand.New(rand.NewSource(time.Now().UnixNano()))
	return w
}

func (w *World) String() string {
	str := fmt.Sprintf("%d nrealm %d (%v) nproc %d (ndone %v) nwork %v maxq %d avgq %v util %v\n schedds:", w.ntick, len(w.realms), w.lambda, w.nproc, w.fairness(), w.nwork, w.maxq, w.avgq/float64(w.ntick), w.util())
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

func (w *World) fairness() []int {
	ndone := make([]int, len(w.realms))
	for _, sd := range w.schedds {
		for i, n := range sd.nproc {
			ndone[i] += n
		}
	}
	return ndone
}

func (w *World) util() float64 {
	u := float64(0)
	for _, sd := range w.schedds {
		u += sd.util
	}
	return u / float64(w.ntick)
}

func (w *World) genLoad() {
	for _, r := range w.realms {
		procs := r.genLoad(w.rand)
		for _, p := range procs {
			q := int(rand.Uint64() % uint64(len(w.procqs)))
			// fmt.Printf("q %d r %d\n", q, p.realm)
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

func uniform(r *rand.Rand) uint64 {
	return (rand.Uint64() % MAX_SERVICE_TIME) + 1
}
