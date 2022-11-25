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
}

func makeClone(mfs *memfssrv.MemFs, fn string, mksessionf MkSessionF) *np.Err {
	cl := &Clone{}
	i, err := mfs.MkDev(fn, cl) // put clone file into root dir
	if err != nil {
		return err
	}
	cl.Inode = i
	cl.mfs = mfs
	cl.mksession = mksessionf
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
	i, err := c.mfs.MkDev(sid.String()+"/"+CTL, s)
	if err != nil {
		return nil, err
	}
	s.Inode = i
	if err := c.mksession(c.mfs, sid); err != nil {
		return nil, err
	}
	return s, nil
}

func (c *Clone) Close(ctx fs.CtxI, m np.Tmode) *np.Err {
	db.DPrintf("CLONEDEV", "%v: Close clone\n", proc.GetProgram())
	return nil
}

func MkCloneDev(mfs *memfssrv.MemFs, fn string, f MkSessionF) error {
	if err := makeClone(mfs, fn, f); err != nil {
		return err
	}
	return nil
}
