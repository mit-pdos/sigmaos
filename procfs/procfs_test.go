package procfs

import (
	"log"
	"testing"

	"github.com/stretchr/testify/assert"

	"sigmaos/ctx"
	"sigmaos/dir"
	"sigmaos/fs"
	"sigmaos/memfs"
	"sigmaos/path"
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

func (ps *Procs) Len() int {
	return len(ps.procs)
}

func TestReadDir(t *testing.T) {
	procs := &Procs{procs: make(map[sp.Tpid]*proc.Proc)}
	p := proc.NewProc("test", nil)
	procs.procs[p.GetPid()] = p

	d := NewProcDir(procs)

	ctx := ctx.NewCtx("", 0, sp.NoClntId, nil, nil)
	root := dir.NewRootDir(ctx, memfs.NewInode, nil)
	err := dir.MkNod(ctx, root, "pids", d)
	assert.Nil(t, err)
	_, o, _, err := root.LookupPath(nil, path.Path{"pids"})
	_, err = o.Open(nil, 0)
	assert.Nil(t, err)
	sts, err := o.(fs.Dir).ReadDir(ctx, 0, 100000)
	assert.Nil(t, err)
	log.Printf("sts %v\n", sts)
}
