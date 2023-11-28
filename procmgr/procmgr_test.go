package procmgr

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

func TestReadDir(t *testing.T) {
	m := make(map[sp.Tpid]*proc.Proc)
	d := NewProcDir(m)
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
