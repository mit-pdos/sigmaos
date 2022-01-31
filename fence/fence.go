package fence

import (
	"fmt"

	np "ulambda/ninep"
)

type Fence struct {
	Fence np.Tfenceid
	Qid   np.Tqid
}

func (f *Fence) String() string {
	return fmt.Sprintf("idf %v qid %v", f.Fence, f.Qid)
}

func MakeFence(fence np.Tfenceid, qid np.Tqid) *Fence {
	return &Fence{fence, qid}
}
