package namesrv

import (
	"fmt"
	"hash/fnv"
	"time"

	"sigmaos/api/fs"
	db "sigmaos/debug"
	"sigmaos/namesrv/fsetcd"
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
	fs    *fsetcd.FsEtcd
	pn    path.Tpathname
	di    fsetcd.DirEntInfo
	mtime int64
}

func newObjDi(fs *fsetcd.FsEtcd, pn path.Tpathname, di fsetcd.DirEntInfo) *Obj {
	o := &Obj{fs: fs, pn: pn, di: di}
	return o
}

func (o *Obj) String() string {
	return fmt.Sprintf("{pn %q di %v (%d)}", o.pn, o.di, o.Dev())
}

func (o *Obj) Path() sp.Tpath {
	return o.di.Path
}

func (o *Obj) Dev() sp.Tdev {
	return sp.DEV_ETCDFS
}

func (o *Obj) Perm() sp.Tperm {
	return o.di.Perm
}

func (o *Obj) IsLeased() bool {
	return o.di.LeaseId.IsLeased()
}

func (o *Obj) Stat(ctx fs.CtxI) (*sp.Tstat, *serr.Err) {
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

func (o *Obj) NewStat() (*sp.Tstat, *serr.Err) {
	st := sp.NewStatNull()
	st.Name = o.pn.Base()
	qid := sp.NewQidPerm(o.di.Perm, 0, o.di.Path)
	st.SetQid(&qid)
	st.SetMode(o.di.Perm)
	st.SetLengthInt(len(o.di.Nf.Data))
	return st, nil
}

func (o *Obj) putObj(f sp.Tfence, data []byte) *serr.Err {
	nf := fsetcd.NewEtcdFile(data)
	c, err := o.fs.PutFile(&o.di, nf, f)
	o.fs.PstatUpdate(o.pn, c)
	return err
}
