package srv

import (
	"sigmaos/api/fs"
	"sigmaos/path"
	"sigmaos/serr"
	"sigmaos/spproto/srv/lockmapv1"
	"sigmaos/spproto/srv/namei"
)

// LookupObj/namei will return an lo and a locked watch for it, even
// in error cases because the caller create a new fid anyway.
func (ps *ProtSrv) lookupObj(ctx fs.CtxI, po *Pobj, target path.Tpathname, ltype lockmapv1.Tlock) ([]fs.FsObj, fs.FsObj, *lockmapv1.PathLock, path.Tpathname, *serr.Err) {
	o := po.Obj()
	lk := ps.plt.Acquire(ctx, o.Path(), ltype)
	if len(target) == 0 {
		return nil, o, lk, nil, nil
	}
	if !o.Perm().IsDir() {
		return nil, o, lk, nil, serr.NewErr(serr.TErrNotDir, po.Pathname().Base())
	}
	return namei.Walk(ps.plt, ctx, o, lk, target, nil, ltype)
}
