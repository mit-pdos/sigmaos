package namei

import (
	"sigmaos/api/fs"
	db "sigmaos/debug"
	"sigmaos/path"
	"sigmaos/serr"
	"sigmaos/spproto/srv/lockmap"
)

// Walk traverses target element by element or in one LookupPath call,
// depending if the underlying file system can do a lookup for the
// complete path.  Caller provides locked dir.
func Walk(plt *lockmap.PathLockTable, ctx fs.CtxI, o fs.FsObj, dlk *lockmap.PathLock, target path.Tpathname, os []fs.FsObj, ltype lockmap.Tlock) ([]fs.FsObj, fs.FsObj, *lockmap.PathLock, path.Tpathname, *serr.Err) {
	d := o.(fs.Dir)
	nos, e, rest, err := d.LookupPath(ctx, target)
	if err != nil { // an error or perhaps a ~
		db.DPrintf(db.NAMEI, "%v: dir %v: file not found %v", ctx.Principal(), d, target[0])
		return os, d, dlk, target, err
	}
	os = append(os, nos...)
	lo := os[len(os)-1]
	if len(rest) == 0 { // done?
		db.DPrintf(db.NAMEI, "%v: namei %q lo %v e %v os %v", ctx.Principal(), target, lo, e, os)
		var flk *lockmap.PathLock
		if target.Base() == "." {
			flk = dlk
		} else {
			flk = plt.Acquire(ctx, lo.Path(), ltype)
			plt.Release(ctx, dlk, ltype)
		}
		return os, e, flk, nil, nil
	}
	switch e := e.(type) {
	case fs.Dir:
		db.DPrintf(db.NAMEI, "%v: namei e %v os(%d) %v target '%v'", ctx.Principal(), e, len(os), os, target[1:])
		dlk = plt.HandOverLock(ctx, dlk, lo.Path(), ltype)
		return Walk(plt, ctx, e, dlk, target[1:], os, ltype)
	default: // an error or perhaps a symlink
		db.DPrintf(db.NAMEI, "%v: error not dir namei %T %v %v %v %v", ctx.Principal(), e, target, d, os, target[1:])
		return os, e, dlk, target, serr.NewErr(serr.TErrNotDir, target[0])
	}
}
