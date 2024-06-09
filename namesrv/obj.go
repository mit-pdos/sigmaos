package namesrv

import (
	"fmt"
	"hash/fnv"
	"time"

	db "sigmaos/debug"
	"sigmaos/fs"
	"sigmaos/fsetcd"
	"sigmaos/path"
	"sigmaos/serr"
	sp "sigmaos/sigmap"
)

func newTpath(pn path.Tpathname) sp.Tpath {
	h := fnv.New64a()
	t := time.Now() // maybe use revision
	h.Write([]byte(pn.String() + t.String()))
	return sp.Tpath(h.Sum64())
}

// An obj is either a directory or file
type Obj struct {
	fs     *fsetcd.FsEtcd
	pn     path.Tpathname
	di     fsetcd.DirEntInfo
	parent sp.Tpath
	mtime  int64
}

func newObjDi(fs *fsetcd.FsEtcd, pn path.Tpathname, di fsetcd.DirEntInfo, parent sp.Tpath) *Obj {
	o := &Obj{fs: fs, pn: pn, di: di, parent: parent}
	return o
}

func (o *Obj) String() string {
	return fmt.Sprintf("pn %q di %v parent %v", o.pn, o.di, o.parent)
}

func (o *Obj) Path() sp.Tpath {
	return o.di.Path
}

func (o *Obj) Perm() sp.Tperm {
	return o.di.Perm
}

func (o *Obj) IsLeased() bool {
	return o.di.LeaseId.IsLeased()
}

// XXX 0 should be o.parent.parent
func (o *Obj) Parent() fs.Dir {
	dir := o.pn.Dir()
	return newDir(newObjDi(o.fs, dir, *fsetcd.NewDirEntInfoDir(o.parent), 0))
}

// XXX SetParent

func (o *Obj) Stat(ctx fs.CtxI) (*sp.Stat, *serr.Err) {
	db.DPrintf(db.NAMED, "Stat: %v\n", o)

	if o.di.Nf == nil {
		nf, _, c, err := o.fs.GetFile(&o.di)
		o.fs.PstatUpdate(o.pn, c)
		if err != nil {
			db.DPrintf(db.NAMED, "Stat: GetFile %v err %v\n", o, err)
			return nil, serr.NewErr(serr.TErrNotfound, o.pn.Base())
		} else {
			o.di.Nf = nf
		}
	}
	st, err := o.NewStat()
	if err != nil {
		return nil, err
	}
	return st, nil
}

func (o *Obj) NewStat() (*sp.Stat, *serr.Err) {
	st := sp.NewStatNull()
	st.Name = o.pn.Base()
	st.SetQid(sp.NewQidPerm(o.di.Perm, 0, o.di.Path))
	st.SetMode(o.di.Perm)
	st.SetLengthInt(len(o.di.Nf.Data))
	return st, nil
}

func (o *Obj) putObj(f sp.Tfence, data []byte) *serr.Err {
	nf := fsetcd.NewEtcdFile(o.di.Perm|0777, data)
	c, err := o.fs.PutFile(&o.di, nf, f)
	o.fs.PstatUpdate(o.pn, c)
	return err
}
