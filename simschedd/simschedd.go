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
	AVG_ARRIVAL_RATE float64 = 0.1 // per tick
)

type Trealm int
type Tftick float64

func (f Tftick) String() string {
	return fmt.Sprintf("%.1fT", f)
}

type Proc struct {
	nTick Tftick
	nMem  Tmem
	r     Trealm
}

func (p *Proc) String() string {
	return fmt.Sprintf("{nT %v nMem %d r %d}", p.nTick, p.nMem, p.r)
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

func (q *Queue) run(n Tftick) {
	ps := make([]*Proc, 0)
	for _, p := range q.q {
		p.nTick -= n
		if p.nTick > 0 {
			ps = append(ps, p)
		}
	}
	q.q = ps
}

func (q *Queue) mem() Tmem {
	m := Tmem(0)
	for _, p := range q.q {
		m += p.nMem
	}
	return m
}

type ProcQ struct {
	qs map[Trealm]*Queue
}

func (pq *ProcQ) String() string {
	str := "["
	for r, q := range pq.qs {
		str += fmt.Sprintf("%d: %v", r, q)
	}
	str += "]"
	return str
}

func (pq *ProcQ) enq(p *Proc) {
	q, ok := pq.qs[p.r]
	if !ok {
		q = newQueue()
		pq.qs[p.r] = q
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

type Schedd struct {
	totMem Tmem
	nMem   Tmem
	q      *Queue
}

func (sd *Schedd) String() string {
	return fmt.Sprintf("{nMem %d q %v}", sd.nMem, sd.q)
}

func (sd *Schedd) mem() Tmem {
	return sd.q.mem()
}

func (sd *Schedd) run() {
	if len(sd.q.q) == 0 {
		return
	}
	n := float64(1.0 / float64(len(sd.q.q)))
	sd.q.run(Tftick(n))
}

type Realm struct {
	realm   Trealm
	poisson *distuv.Poisson
}

func newRealm(lambda float64) *Realm {
	r := &Realm{}
	r.poisson = &distuv.Poisson{Lambda: lambda}
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
}

func newWorld(nProcQ, nSchedd int) *World {
	w := &World{}
	w.schedds = make([]*Schedd, nSchedd)
	w.procqs = make([]*ProcQ, nProcQ)
	for i := 0; i < len(w.schedds); i++ {
		w.schedds[i] = &Schedd{totMem: MAX_MEM, q: newQueue()}
	}
	for i := 0; i < len(w.procqs); i++ {
		w.procqs[i] = &ProcQ{qs: make(map[Trealm]*Queue)}
	}
	w.realms = make([]*Realm, 1)
	w.realms[0] = newRealm(AVG_ARRIVAL_RATE)
	w.rand = rand.New(rand.NewSource(time.Now().UnixNano()))
	return w
}

func (w *World) String() string {
	str := fmt.Sprintf("%d\n schedds:", w.ntick)
	for _, sd := range w.schedds {
		str += sd.String()
	}
	str += "\n procQs:"
	for _, pq := range w.procqs {
		str += pq.String()
	}
	return str
}

func (w *World) genLoad() {
	for _, r := range w.realms {
		procs := r.genLoad(w.rand)
		for _, p := range procs {
			q := int(rand.Uint64() % uint64(len(w.procqs)))
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

func (w *World) Tick() {
	w.ntick += 1
	w.genLoad()
	fmt.Printf("w0 %v\n", w)
	w.getProcs()
	w.compute()
	fmt.Printf("w1 %v\n", w)
}

func zipf(r *rand.Rand) uint64 {
	z := rand.NewZipf(r, 2.0, 1.0, MAX_SERVICE_TIME-1)
	return z.Uint64() + 1
}

func uniform(r *rand.Rand) uint64 {
	return (rand.Uint64() % MAX_SERVICE_TIME) + 1
}
