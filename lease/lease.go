package lease

import (
	"fmt"
	"log"
	"sync"

	db "ulambda/debug"
	np "ulambda/ninep"
)

type Lease struct {
	Fn  []string
	Qid np.Tqid
}

func MakeLease(lease []string, qid np.Tqid) *Lease {
	return &Lease{lease, qid}
}

func (lease *Lease) Check(qid np.Tqid) error {
	// log.Printf("%v: check lease %v %v\n", db.GetName(), lease.Qid, qid)
	if qid != lease.Qid {
		return fmt.Errorf("stale lease %v", np.Join(lease.Fn))
	}
	return nil
}

//
//  Map of leases indexed by pathname of lease
//

type LeaseMap struct {
	sync.Mutex
	leases map[string]*Lease
}

func MakeLeaseMap() *LeaseMap {
	lm := &LeaseMap{}
	lm.leases = make(map[string]*Lease)
	return lm
}

func (lm *LeaseMap) Add(l *Lease) error {
	lm.Lock()
	defer lm.Unlock()

	if l == nil {
		log.Fatalf("%v: Add nil lease\n", db.GetName(), l)
	}

	fn := np.Join(l.Fn)
	if _, ok := lm.leases[fn]; ok {
		return fmt.Errorf("%v already leased", fn)
	}
	lm.leases[fn] = l
	return nil
}

func (lm *LeaseMap) Present(path []string) bool {
	lm.Lock()
	defer lm.Unlock()

	fn := np.Join(path)
	_, ok := lm.leases[fn]
	return ok
}

func (lm *LeaseMap) Del(path []string) error {
	lm.Lock()
	defer lm.Unlock()

	fn := np.Join(path)
	if _, ok := lm.leases[fn]; !ok {
		return fmt.Errorf("%v no lease", fn)
	}
	delete(lm.leases, fn)
	return nil
}

func (lm *LeaseMap) Leases() []*Lease {
	lm.Lock()
	defer lm.Unlock()

	leases := make([]*Lease, 0, len(lm.leases))
	for _, l := range lm.leases {
		leases = append(leases, l)
	}
	log.Printf("leases %v\n", leases)
	return leases

}
