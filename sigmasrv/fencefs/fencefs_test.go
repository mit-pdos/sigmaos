package fencefs_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"sigmaos/ctx"
	"sigmaos/sigmasrv/fencefs"
	sp "sigmaos/sigmap"
)

func TestNewFenceFs(t *testing.T) {
	fence := sp.Tfence{}
	fence.Epoch = 10

	ctx := ctx.NewCtxNull()
	root := fencefs.NewRoot(ctx, nil)
	assert.NotNil(t, root)

	i, err := root.Create(ctx, fence.PathName, 0777, sp.OWRITE, sp.NoLeaseId, sp.NoFence(), nil)
	assert.Nil(t, err)
	assert.NotNil(t, i)
}
