package clonedev

import (
	"path/filepath"
	"sync"

	db "sigmaos/debug"
	"sigmaos/fs"
	"sigmaos/inode"
	"sigmaos/memfssrv"
	"sigmaos/serr"
	"sigmaos/sessdev"
	"sigmaos/sessp"
	sp "sigmaos/sigmap"
	sps "sigmaos/sigmaprotsrv"
)

type NewSessionF func(*memfssrv.MemFs, sessp.Tsession) *serr.Err
type WriteCtlF func(sessp.Tsession, fs.CtxI, sp.Toffset, []byte, sp.Tfence) (sp.Tsize, *serr.Err)

type Clone struct {
	mu sync.Mutex
	*inode.Inode
	mfs        *memfssrv.MemFs
	newsession NewSessionF
	detach     sps.DetachSessF
	dir        string
	wctl       WriteCtlF
}

// Make a Clone dev inode in directory <dir> in memfs
func newClone(mfs *memfssrv.MemFs, dir string, news NewSessionF, d sps.DetachSessF, w WriteCtlF) *serr.Err {
	cl := &Clone{
		Inode:      mfs.NewDevInode(),
		mfs:        mfs,
		newsession: news,
		detach:     d,
		dir:        dir,
		wctl:       w,
	}
	pn := dir + "/" + sessdev.CLONE
	db.DPrintf(db.CLONEDEV, "newClone %q\n", dir)
	err := mfs.MkNod(pn, cl) // put clone file into dir <dir>
	if err != nil {
		return err
	}
	return nil
}

func (c *Clone) Stat(ctx fs.CtxI) (*sp.Stat, *serr.Err) {
	st, err := c.Inode.NewStat()
	if err != nil {
		return nil, err
	}
	return st, nil
}

// XXX clean up in case of error
func (c *Clone) Open(ctx fs.CtxI, m sp.Tmode) (fs.FsObj, *serr.Err) {
	c.mu.Lock()
	defer c.mu.Unlock()

	sid := ctx.SessionId()
	pn := filepath.Join(c.dir, sid.String())
	db.DPrintf(db.CLONEDEV, "Clone create %q\n", pn)
	_, err := c.mfs.Create(pn, sp.DMDIR, sp.ORDWR, sp.NoLeaseId)
	if err != nil && err.Code() != serr.TErrExists {
		db.DPrintf(db.CLONEDEV, "MkDir %q err %v\n", pn, err)
		return nil, err
	}
	var s *session
	ctl := pn + "/" + sessdev.CTL
	if err == nil {
		s = &session{id: sid, wctl: c.wctl}
		s.Inode = c.mfs.NewDevInode()
		if err := c.mfs.MkNod(ctl, s); err != nil {
			db.DPrintf(db.CLONEDEV, "NewDev %q err %v\n", ctl, err)
			return nil, err
		}
		if err := c.mfs.RegisterDetachSess(c.Detach, sid); err != nil {
			db.DPrintf(db.CLONEDEV, "RegisterDetach err %v\n", err)
		}
		if err := c.newsession(c.mfs, sid); err != nil {
			return nil, err
		}
	} else {
		lo, err := c.mfs.Open(ctl, sp.OREAD)
		if err != nil {
			db.DPrintf(db.CLONEDEV, "open %q err %v\n", ctl, err)
			return nil, err
		}
		s = lo.(*session)
	}
	return s, nil
}

func (c *Clone) Close(ctx fs.CtxI, m sp.Tmode) *serr.Err {
	sid := ctx.SessionId().String()
	db.DPrintf(db.CLONEDEV, "Close %q\n", sid)
	return nil
}

func (c *Clone) Detach(session sessp.Tsession) {
	db.DPrintf(db.CLONEDEV, "Detach %v\n", session)
	dir := filepath.Join(c.dir, session.String())
	ctl := filepath.Join(dir, sessdev.CTL)
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

func NewCloneDev(mfs *memfssrv.MemFs, dir string, f NewSessionF, d sps.DetachSessF, w WriteCtlF) error {
	if err := newClone(mfs, dir, f, d, w); err != nil {
		return err
	}
	return nil
}
