package fencefs_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"sigmaos/ctx"
	"sigmaos/fencefs"
	sp "sigmaos/sigmap"
)

func TestMakeFenceFs(t *testing.T) {
	fence := sp.Tfence1{}
	fence.Epoch = 10

	ctx := ctx.MkCtx("", 0, nil)
	root := fencefs.MakeRoot()
	assert.NotNil(t, root)
	i, err := root.Create(ctx, fence.FenceId.Path.String(), 0777, sp.OWRITE)
	assert.Nil(t, err)
	assert.NotNil(t, i)
}
