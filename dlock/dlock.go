package dlock

import (
	"fmt"
	"log"

	db "ulambda/debug"
	np "ulambda/ninep"
)

type Dlock struct {
	Fn  []string
	Qid np.Tqid
}

func MakeDlock(dlock []string, qid np.Tqid) *Dlock {
	return &Dlock{dlock, qid}
}

func (dlock *Dlock) Check(qid np.Tqid) error {
	log.Printf("%v: check dlock %v %v\n", db.GetName(), dlock.Qid, qid)
	if qid != dlock.Qid {
		return fmt.Errorf("dlock %v is stale", dlock.Fn)
	}
	return nil
}
