package lockmap_test

import (
	"testing"

	"sigmaos/ctx"
	"sigmaos/path"
	"sigmaos/spproto/srv/lockmap"
)

func TestCompile(t *testing.T) {
}

func BenchmarkLock(b *testing.B) {
	plt := lockmap.NewPathLockTable()
	path := path.Split("aaa/bbb/ccc")
	ctx := ctx.NewCtxNull()
	for i := 0; i < b.N; i++ {
		pl := plt.Acquire(ctx, path, lockmap.WLOCK)
		plt.Release(ctx, pl, lockmap.WLOCK)
	}
}
