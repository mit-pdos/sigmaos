package protsrv

import (
	db "ulambda/debug"
	"ulambda/fid"
	"ulambda/fs"
	np "ulambda/ninep"
	"ulambda/watch"
)

// If an lo is returned, namei will return a locked watch for it
func (ps *ProtSrv) namei(ctx fs.CtxI, o fs.FsObj, src, target np.Path, os []fs.FsObj) ([]fs.FsObj, fs.FsObj, *watch.Watch, np.Path, *np.Err) {
	dws := ps.wt.WatchLookupL(src)
	d := o.(fs.Dir)
	e, err := d.Lookup(ctx, target[0])
	if err != nil {
		db.DPrintf("PROTSRV", "%v: dir %v: file not found %v", ctx.Uname(), d, target[0])
		return os, d, dws, target, err
	}
	os = append(os, e)
	if len(target) == 1 { // done?
		pn := src.Append(target[0])
		db.DPrintf("PROTSRV", "%v: namei %v e %v os %v", ctx.Uname(), pn, e, os)
		ews := ps.wt.WatchLookupL(pn)
		ps.wt.Release(dws)
		return os, e, ews, nil, nil
	}
	switch e := e.(type) {
	case fs.Dir:
		ps.wt.Release(dws) // for "."
		return ps.namei(ctx, e, src.Append(target[0]), target[1:], os)
	default:
		db.DPrintf("PROTSRV", "%v: error not dir namei %T %v %v %v %v", ctx.Uname(), e, target, d, os, target[1:])
		return os, e, dws, target, np.MkErr(np.TErrNotDir, target[0])
	}
}

func (ps *ProtSrv) lookupObj(ctx fs.CtxI, po *fid.Pobj, target np.Path) ([]fs.FsObj, fs.FsObj, *watch.Watch, np.Path, *np.Err) {
	if len(target) == 0 {
		return nil, nil, nil, nil, nil
	}
	o := po.Obj()
	src := po.Path().Copy()
	if !o.Perm().IsDir() {
		return nil, nil, nil, nil, np.MkErr(np.TErrNotDir, src.Base())
	}
	return ps.namei(ctx, o, src, target, nil)
}
