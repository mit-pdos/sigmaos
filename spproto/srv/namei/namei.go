package namei

import (
	"sigmaos/api/fs"
	db "sigmaos/debug"
	"sigmaos/path"
	"sigmaos/serr"
	"sigmaos/spproto/srv/lockmapv1"
)

// Walk traverses target element by element or in one LookupPath call,
// depending if the underlying file system can do a lookup for the
// complete path.  Caller provides locked dir.
func Walk(plt *lockmapv1.PathLockTable, ctx fs.CtxI, o fs.FsObj, dlk *lockmapv1.PathLock, target path.Tpathname, os []fs.FsObj, ltype lockmapv1.Tlock) ([]fs.FsObj, fs.FsObj, *lockmapv1.PathLock, path.Tpathname, *serr.Err) {
	d := o.(fs.Dir)
	nos, e, rest, err := d.LookupPath(ctx, target)
	if err != nil { // an error or perhaps a ~
		db.DPrintf(db.NAMEI, "%v: dir %v: file not found %v", ctx.Principal(), d, target[0])
		// releaseLk(plt, ctx, plk, ltype)
		return os, d, dlk, target, err
	}
	os = append(os, nos...)
	lo := os[len(os)-1]
	if len(rest) == 0 { // done?
		db.DPrintf(db.NAMEI, "%v: namei %v e %v os %v", ctx.Principal(), lo, e, os)
		flk := plt.Acquire(ctx, lo.Path(), ltype)
		plt.Release(ctx, dlk, ltype)
		// releaseLk(plt, ctx, plk, ltype)
		return os, e, flk, nil, nil
	}
	// releaseLk(plt, ctx, plk, ltype)
	switch e := e.(type) {
	case fs.Dir:
		dlk = plt.HandOverLock(ctx, dlk, lo.Path(), ltype)
		return Walk(plt, ctx, e, dlk, target[1:], os, ltype)
	default: // an error or perhaps a symlink
		db.DPrintf(db.NAMEI, "%v: error not dir namei %T %v %v %v %v", ctx.Principal(), e, target, d, os, target[1:])
		return os, e, dlk, target, serr.NewErr(serr.TErrNotDir, target[0])
	}
}
