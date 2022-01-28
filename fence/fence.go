package fence

import (
	"fmt"

	np "ulambda/ninep"
)

type Fence struct {
	Path string
	Qid  np.Tqid
}

func (f *Fence) String() string {
	return fmt.Sprintf("p %v qid %v", f.Path, f.Qid)
}

func MakeFence(path string, qid np.Tqid) *Fence {
	return &Fence{path, qid}
}
