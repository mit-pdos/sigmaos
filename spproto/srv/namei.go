package srv

import (
	"time"

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
func (ps *ProtSrv) lookupObj(ctx fs.CtxI, f *fid.Fid, target path.Tpathname, ltype lockmap.Tlock) ([]fs.FsObj, fs.FsObj, *lockmap.PathLock, string, *serr.Err) {
	db.DPrintf(db.NAMEI, "%v: lookupObj %v target '%v'", ctx.Principal(), f, target)
	o := f.Obj()
	name := f.Name()
	lk := ps.plt.Acquire(ctx, f.Path(), ltype)
	if len(target) == 0 {
		return nil, o, lk, name, nil
	}
	if !o.Perm().IsDir() {
		return nil, o, lk, "", serr.NewErr(serr.TErrNotDir, f.Name())
	}
	s := time.Now()
	os, lo, lk, _, err := namei.Walk(ps.plt, ctx, o, lk, target, nil, ltype)
	if err == nil {
		name = target[len(os)-1]
	}
	if db.WillBePrinted(db.WALK_LAT) {
		db.DPrintf(db.WALK_LAT, "ProtSrv lookupObj namei.Walk %v %v lat %v", f.Ctx().ClntId(), target, time.Since(s))
	}
	return os, lo, lk, name, err
}
