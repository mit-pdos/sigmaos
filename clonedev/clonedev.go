package clonedev

import (
	db "sigmaos/debug"
	"sigmaos/fs"
	"sigmaos/inode"
	"sigmaos/memfssrv"
	np "sigmaos/ninep"
	"sigmaos/proc"
)

const (
	CTL = "ctl"
)

type MkSessionF func(*memfssrv.MemFs, np.Tsession) *np.Err

type Clone struct {
	*inode.Inode
	mfs       *memfssrv.MemFs
	mksession MkSessionF
	detach    np.DetachF
}

func makeClone(mfs *memfssrv.MemFs, fn string, mks MkSessionF, d np.DetachF) *np.Err {
	cl := &Clone{}
	cl.Inode = mfs.MakeDevInode()
	err := mfs.MkDev(fn, cl) // put clone file into root dir
	if err != nil {
		return err
	}
	cl.mfs = mfs
	cl.mksession = mks
	cl.detach = d
	return nil
}

// XXX clean up in case of error
func (c *Clone) Open(ctx fs.CtxI, m np.Tmode) (fs.FsObj, *np.Err) {
	sid := ctx.SessionId()
	db.DPrintf("CLONEDEV", "%v: Open clone dir %v\n", proc.GetProgram(), sid)
	if _, err := c.mfs.Create(sid.String(), np.DMDIR, np.ORDWR); err != nil {
		return nil, err
	}
	s := &session{}
	s.id = sid
	s.Inode = c.mfs.MakeDevInode()
	err := c.mfs.MkDev(sid.String()+"/"+CTL, s)
	if err != nil {
		return nil, err
	}
	if err := c.mfs.RegisterDetach(c.Detach, sid); err != nil {
		db.DPrintf("CLONEDEV", "%v: RegisterDetach err %v\n", proc.GetProgram(), err)
	}
	if err := c.mksession(c.mfs, sid); err != nil {
		return nil, err
	}
	return s, nil
}

func (c *Clone) Close(ctx fs.CtxI, m np.Tmode) *np.Err {
	db.DPrintf("CLONEDEV", "%v: Close clone\n", proc.GetProgram())
	return nil
}

func (c *Clone) Detach(session np.Tsession) {
	db.DPrintf("CLONEDEV", "Detach %v\n", session)
	dir := session.String() + "/"
	if err := c.mfs.Remove(dir + CTL); err != nil {
		db.DPrintf("CLONEDEV", "Remove ctl err %v\n", err)
	}
	if c.detach != nil {
		c.detach(session)
	}
	if err := c.mfs.Remove(dir); err != nil {
		db.DPrintf("CLONEDEV", "Detach err %v\n", err)
	}
}

func MkCloneDev(mfs *memfssrv.MemFs, fn string, f MkSessionF, d np.DetachF) error {
	if err := makeClone(mfs, fn, f, d); err != nil {
		return err
	}
	return nil
}
