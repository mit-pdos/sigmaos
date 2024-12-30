package srv

import (
	"sigmaos/api/fs"
	"sigmaos/path"
	"sigmaos/serr"
	"sigmaos/spproto/srv/lockmap"
	"sigmaos/spproto/srv/namei"
)

// LookupObj/namei will return an lo and a locked watch for it, even
// in error cases because the caller create a new fid anyway.
func (ps *ProtSrv) lookupObj(ctx fs.CtxI, po *Pobj, target path.Tpathname, ltype lockmap.Tlock) ([]fs.FsObj, fs.FsObj, *lockmap.PathLock, path.Tpathname, *serr.Err) {
	src := po.Pathname()
	lk := ps.plt.Acquire(ctx, src, ltype)
	o := po.Obj()
	if len(target) == 0 {
		return nil, o, lk, nil, nil
	}
	if !o.Perm().IsDir() {
		return nil, o, lk, nil, serr.NewErr(serr.TErrNotDir, src.Base())
	}
	return namei.Walk(ps.plt, ctx, o, lk, src, target, nil, ltype)
}
