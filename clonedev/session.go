package clonedev

import (
	db "sigmaos/debug"
	"sigmaos/sessp"
	"sigmaos/fs"
	"sigmaos/inode"
	sp "sigmaos/sigmap"
)

type session struct {
	*inode.Inode
	id sessp.Tsession
}

func (s *session) Read(ctx fs.CtxI, off sp.Toffset, cnt sessp.Tsize, v sp.TQversion) ([]byte, *sessp.Err) {
	if off > 0 {
		return nil, nil
	}
	return []byte(s.id.String()), nil
}

func (s *session) Write(ctx fs.CtxI, off sp.Toffset, b []byte, v sp.TQversion) (sessp.Tsize, *sessp.Err) {
	return 0, sessp.MkErr(sessp.TErrNotSupported, nil)
}

func (s *session) Close(ctx fs.CtxI, m sp.Tmode) *sessp.Err {
	db.DPrintf(db.CLONEDEV, "Close session ctl %v\n", s.id)
	return nil
}
