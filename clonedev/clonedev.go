package clonedev

import (
	db "sigmaos/debug"
	"sigmaos/fs"
	"sigmaos/inode"
	"sigmaos/memfssrv"
	"sigmaos/proc"
	"sigmaos/serr"
	"sigmaos/sessp"
	sp "sigmaos/sigmap"
)

const (
	CLONE = "clone-"
	CTL   = "ctl"
)

type MkSessionF func(*memfssrv.MemFs, sessp.Tsession) *serr.Err
type WriteCtlF func(sessp.Tsession, fs.CtxI, sp.Toffset, []byte, sp.TQversion) (sessp.Tsize, *serr.Err)

func SidName(sid string, fn string) string {
	return sid + "-" + fn
}

func CloneName(fn string) string {
	return CLONE + fn
}

type Clone struct {
	*inode.Inode
	mfs       *memfssrv.MemFs
	mksession MkSessionF
	detach    sp.DetachF
	fn        string
	wctl      WriteCtlF
}

func makeClone(mfs *memfssrv.MemFs, fn string, mks MkSessionF, d sp.DetachF, w WriteCtlF) *serr.Err {
	cl := &Clone{}
	cl.Inode = mfs.MakeDevInode()
	err := mfs.MkDev(CloneName(fn), cl) // put clone file into root dir
	if err != nil {
		return err
	}
	cl.mfs = mfs
	cl.mksession = mks
	cl.detach = d
	cl.fn = fn
	cl.wctl = w
	return nil
}

// XXX clean up in case of error
func (c *Clone) Open(ctx fs.CtxI, m sp.Tmode) (fs.FsObj, *serr.Err) {
	sid := ctx.SessionId()
	n := SidName(sid.String(), c.fn)
	db.DPrintf(db.CLONEDEV, "%v: Clone create %v\n", proc.GetName(), n)
	_, err := c.mfs.Create(n, sp.DMDIR, sp.ORDWR)
	if err != nil && err.Code() != serr.TErrExists {
		db.DPrintf(db.CLONEDEV, "%v: MkDir %v err %v\n", proc.GetName(), n, err)
		return nil, err
	}
	var s *session
	ctl := n + "/" + CTL
	if err == nil {
		s = &session{id: sid, wctl: c.wctl}
		s.Inode = c.mfs.MakeDevInode()
		if err := c.mfs.MkDev(ctl, s); err != nil {
			db.DPrintf(db.CLONEDEV, "%v: MkDev %v err %v\n", proc.GetName(), ctl, err)
			return nil, err
		}
		if err := c.mfs.RegisterDetach(c.Detach, sid); err != nil {
			db.DPrintf(db.CLONEDEV, "%v: RegisterDetach err %v\n", proc.GetName(), err)
		}
		if err := c.mksession(c.mfs, sid); err != nil {
			return nil, err
		}
	} else {
		lo, err := c.mfs.Open(ctl, sp.OREAD)
		s = lo.(*session)
		if err != nil {
			db.DPrintf(db.CLONEDEV, "%v: open %v err %v\n", proc.GetName(), ctl, err)
			return nil, err
		}
	}
	return s, nil
}

func (c *Clone) Close(ctx fs.CtxI, m sp.Tmode) *serr.Err {
	sid := SidName(ctx.SessionId().String(), c.fn)
	db.DPrintf(db.CLONEDEV, "%v: Close %v\n", proc.GetName(), sid)
	return nil
}

func (c *Clone) Detach(session sessp.Tsession) {
	db.DPrintf(db.CLONEDEV, "Detach %v\n", session)
	dir := SidName(session.String(), c.fn)
	n := dir + "/" + CTL
	if err := c.mfs.Remove(n); err != nil {
		db.DPrintf(db.CLONEDEV, "Remove %v err %v\n", n, err)
	}
	if c.detach != nil {
		c.detach(session)
	}
	if err := c.mfs.Remove(dir); err != nil {
		db.DPrintf(db.CLONEDEV, "Detach err %v\n", err)
	}
}

func MkCloneDev(mfs *memfssrv.MemFs, fn string, f MkSessionF, d sp.DetachF, w WriteCtlF) error {
	if err := makeClone(mfs, fn, f, d, w); err != nil {
		return err
	}
	return nil
}
