package srv

import (
	"sigmaos/api/fs"
	db "sigmaos/debug"
	"sigmaos/path"
	"sigmaos/serr"
	"sigmaos/spproto/srv/fid"
	"sigmaos/spproto/srv/lockmap"
	"sigmaos/spproto/srv/namei"
)

func getParent(start fs.Dir, os []fs.FsObj) fs.Dir {
	if len(os) <= 1 {
		return start
	} else {
		d := os[len(os)-2]
		return d.(fs.Dir)
	}
}

// LookupObj/namei will return an lo and a locked watch for it, even
// in error cases because the caller create a new fid anyway.
func (ps *ProtSrv) lookupObj(ctx fs.CtxI, po *fid.Pobj, target path.Tpathname, ltype lockmap.Tlock) ([]fs.FsObj, fs.FsObj, *lockmap.PathLock, path.Tpathname, *serr.Err) {
	db.DPrintf(db.NAMEI, "%v: lookupObj %v target '%v'", ctx.Principal(), po, target)
	o := po.Obj()
	lk := ps.plt.Acquire(ctx, o.Path(), ltype)
	if len(target) == 0 {
		return nil, o, lk, nil, nil
	}
	if !o.Perm().IsDir() {
		return nil, o, lk, nil, serr.NewErr(serr.TErrNotDir, po.Name())
	}
	return namei.Walk(ps.plt, ctx, o, lk, target, nil, ltype)
}
