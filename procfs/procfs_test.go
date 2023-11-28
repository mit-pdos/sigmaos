package procfs

import (
	"log"
	"testing"

	"github.com/stretchr/testify/assert"

	"sigmaos/ctx"
	"sigmaos/dir"
	"sigmaos/memfs"
	"sigmaos/proc"
	sp "sigmaos/sigmap"
)

type Procs struct {
	procs map[sp.Tpid]*proc.Proc
}

func (ps *Procs) GetProcs() []*proc.Proc {
	procs := make([]*proc.Proc, 0, len(ps.procs))
	for _, p := range ps.procs {
		procs = append(procs, p)
	}
	return procs
}

func (ps *Procs) Lookup(n string) (*proc.Proc, bool) {
	if p, ok := ps.procs[sp.Tpid(n)]; ok {
		return p, ok
	}
	return nil, false
}

func TestReadDir(t *testing.T) {
	procs := &Procs{procs: make(map[sp.Tpid]*proc.Proc)}
	d := NewProcDir(procs)
	log.Printf("dir %T\n", d)

	ctx := ctx.NewCtx("", 0, sp.NoClntId, nil, nil)
	root := dir.NewRootDir(ctx, memfs.NewInode, nil)
	err := dir.MkNod(ctx, root, "pids", d)
	assert.Nil(t, err)
	log.Printf("dir %T\n", root)

	sts, err := root.ReadDir(ctx, 0, 100000)
	assert.Nil(t, err)
	log.Printf("dir %v\n", sts)
}
