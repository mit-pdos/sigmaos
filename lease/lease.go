package lease

import (
	"fmt"

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
		return fmt.Errorf("lease %v is stale", lease.Fn)
	}
	return nil
}
