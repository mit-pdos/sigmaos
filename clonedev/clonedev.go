package clonedev

import (
	"path"

	db "sigmaos/debug"
	"sigmaos/fs"
	"sigmaos/inode"
	"sigmaos/memfssrv"
	"sigmaos/proc"
	"sigmaos/serr"
	"sigmaos/sessdev"
	"sigmaos/sessp"
	sp "sigmaos/sigmap"
	sps "sigmaos/sigmaprotsrv"
)

type MkSessionF func(*memfssrv.MemFs, sessp.Tsession) *serr.Err
type WriteCtlF func(sessp.Tsession, fs.CtxI, sp.Toffset, []byte, sp.TQversion, sp.Tfence) (sp.Tsize, *serr.Err)

type Clone struct {
	*inode.Inode
	mfs       *memfssrv.MemFs
	mksession MkSessionF
	detach    sps.DetachSessF
	dir       string
	wctl      WriteCtlF
}

// Make a Clone dev inode in directory <dir> in memfs
func makeClone(mfs *memfssrv.MemFs, dir string, mks MkSessionF, d sps.DetachSessF, w WriteCtlF) *serr.Err {
	cl := &Clone{
		Inode:     mfs.MakeDevInode(),
		mfs:       mfs,
		mksession: mks,
		detach:    d,
		dir:       dir,
		wctl:      w,
	}
	pn := dir + "/" + sessdev.CLONE
	db.DPrintf(db.CLONEDEV, "makeClone %q\n", dir)
	err := mfs.MkDev(pn, cl) // put clone file into dir <dir>
	if err != nil {
		return err
	}
	return nil
}

// XXX clean up in case of error
func (c *Clone) Open(ctx fs.CtxI, m sp.Tmode) (fs.FsObj, *serr.Err) {
	sid := ctx.SessionId()
	pn := path.Join(c.dir, sid.String())
	db.DPrintf(db.CLONEDEV, "%v: Clone create %q\n", proc.GetName(), pn)
	_, err := c.mfs.Create(pn, sp.DMDIR, sp.ORDWR, sp.NoLeaseId)
	if err != nil && err.Code() != serr.TErrExists {
		db.DPrintf(db.CLONEDEV, "%v: MkDir %q err %v\n", proc.GetName(), pn, err)
		return nil, err
	}
	var s *session
	ctl := pn + "/" + sessdev.CTL
	if err == nil {
		s = &session{id: sid, wctl: c.wctl}
		s.Inode = c.mfs.MakeDevInode()
		if err := c.mfs.MkDev(ctl, s); err != nil {
			db.DPrintf(db.CLONEDEV, "%v: MkDev %q err %v\n", proc.GetName(), ctl, err)
			return nil, err
		}
		if err := c.mfs.RegisterDetachSess(c.Detach, sid); err != nil {
			db.DPrintf(db.CLONEDEV, "%v: RegisterDetach err %v\n", proc.GetName(), err)
		}
		if err := c.mksession(c.mfs, sid); err != nil {
			return nil, err
		}
	} else {
		lo, err := c.mfs.Open(ctl, sp.OREAD)
		s = lo.(*session)
		if err != nil {
			db.DPrintf(db.CLONEDEV, "%v: open %q err %v\n", proc.GetName(), ctl, err)
			return nil, err
		}
	}
	return s, nil
}

func (c *Clone) Close(ctx fs.CtxI, m sp.Tmode) *serr.Err {
	sid := ctx.SessionId().String()
	db.DPrintf(db.CLONEDEV, "%v: Close %q\n", proc.GetName(), sid)
	return nil
}

func (c *Clone) Detach(session sessp.Tsession) {
	db.DPrintf(db.CLONEDEV, "Detach %v\n", session)
	dir := path.Join(c.dir, session.String())
	ctl := path.Join(dir, sessdev.CTL)
	if err := c.mfs.Remove(ctl); err != nil {
		db.DPrintf(db.CLONEDEV, "Remove %v err %v\n", ctl, err)
	}
	if c.detach != nil {
		c.detach(session)
	}
	if err := c.mfs.Remove(dir); err != nil {
		db.DPrintf(db.CLONEDEV, "Detach err %v\n", err)
	}
}

func MkCloneDev(mfs *memfssrv.MemFs, dir string, f MkSessionF, d sps.DetachSessF, w WriteCtlF) error {
	if err := makeClone(mfs, dir, f, d, w); err != nil {
		return err
	}
	return nil
}
