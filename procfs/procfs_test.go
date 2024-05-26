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
	_, o, _, err := root.LookupPath(nil, path.Tpathname{"pids"})
	_, err = o.Open(nil, 0)
	assert.Nil(t, err)
	pids := o.(fs.Dir)
	sts, err := pids.ReadDir(ctx, 0, 100000)
	assert.Nil(t, err)
	for _, st := range sts {
		_, o, _, err := pids.LookupPath(nil, path.Tpathname{st.Name})
		assert.Nil(t, err)
		pid := o.(fs.File)
		b, err := pid.Read(ctx, 0, 10000, sp.NoFence())
		assert.Nil(t, err)
		log.Printf("b %v\n", string(b))
	}
}
