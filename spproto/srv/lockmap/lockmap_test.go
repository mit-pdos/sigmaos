package lockmap_test

import (
	"testing"

	"sigmaos/ctx"
	sp "sigmaos/sigmap"
	"sigmaos/spproto/srv/lockmap"
)

func TestCompile(t *testing.T) {
}

func BenchmarkLock(b *testing.B) {
	plt := lockmap.NewPathLockTable()
	ctx := ctx.NewCtxNull()
	for i := 0; i < b.N; i++ {
		pl := plt.Acquire(ctx, sp.Tpath(i), lockmap.WLOCK)
		plt.Release(ctx, pl, lockmap.WLOCK)
	}
}
