package lockmapv1_test

import (
	"testing"

	"sigmaos/ctx"
	sp "sigmaos/sigmap"
	"sigmaos/spproto/srv/lockmapv1"
)

func TestCompile(t *testing.T) {
}

func BenchmarkLock(b *testing.B) {
	plt := lockmapv1.NewPathLockTable()
	ctx := ctx.NewCtxNull()
	for i := 0; i < b.N; i++ {
		pl := plt.Acquire(ctx, sp.Tpath(i), lockmapv1.WLOCK)
		plt.Release(ctx, pl, lockmapv1.WLOCK)
	}
}
