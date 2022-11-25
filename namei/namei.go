package namei

import (
	db "sigmaos/debug"
	"sigmaos/fs"
	"sigmaos/lockmap"
	np "sigmaos/ninep"
)

func releaseLk(plt *lockmap.PathLockTable, ctx fs.CtxI, pl *lockmap.PathLock) {
	if pl != nil {
		plt.Release(ctx, pl)
	}
}

// Walk traverses target element by element or in one LookupPath call,
// depending if the underlying file system can do a lookup for the
// complete path.  Caller provides locked dir.
func Walk(plt *lockmap.PathLockTable, ctx fs.CtxI, o fs.FsObj, dlk *lockmap.PathLock, dn, target np.Path, os []fs.FsObj) ([]fs.FsObj, fs.FsObj, *lockmap.PathLock, np.Path, *np.Err) {
	// ps.stats.IncPathString(dlk.Path())
	fn := dn.AppendPath(target)
	var plk *lockmap.PathLock
	if len(target) > 1 {
		// lock parent directory
		plk = plt.Acquire(ctx, fn.Dir())
	}
	d := o.(fs.Dir)
	nos, e, rest, err := d.LookupPath(ctx, target)
	if err != nil { // an error or perhaps a ~
		db.DPrintf("NAMEI", "%v: dir %v: file not found %v", ctx.Uname(), d, target[0])
		releaseLk(plt, ctx, plk)
		return os, d, dlk, target, err
	}
	os = append(os, nos...)
	if len(rest) == 0 { // done?
		db.DPrintf("NAMEI", "%v: namei %v e %v os %v", ctx.Uname(), fn, e, os)
		flk := plt.Acquire(ctx, fn)
		plt.Release(ctx, dlk)
		releaseLk(plt, ctx, plk)
		return os, e, flk, nil, nil
	}
	releaseLk(plt, ctx, plk)
	switch e := e.(type) {
	case fs.Dir:
		dlk = plt.HandOverLock(ctx, dlk, target[0])
		return Walk(plt, ctx, e, dlk, dn.Append(target[0]), target[1:], os)
	default: // an error or perhaps a symlink
		db.DPrintf("NAMEI", "%v: error not dir namei %T %v %v %v %v", ctx.Uname(), e, target, d, os, target[1:])
		return os, e, dlk, target, np.MkErr(np.TErrNotDir, target[0])
	}
}
