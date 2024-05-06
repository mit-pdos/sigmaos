package clonedev

import (
	db "sigmaos/debug"
	"sigmaos/fs"
	"sigmaos/inode"
	"sigmaos/serr"
	"sigmaos/sessp"
	sp "sigmaos/sigmap"
)

type session struct {
	*inode.Inode
	id   sessp.Tsession
	wctl WriteCtlF
}

func (s *session) Stat(ctx fs.CtxI) (*sp.Stat, *serr.Err) {
	st, err := s.Inode.NewStat()
	if err != nil {
		return nil, err
	}
	return st, nil
}

func (s *session) Read(ctx fs.CtxI, off sp.Toffset, cnt sp.Tsize, f sp.Tfence) ([]byte, *serr.Err) {
	if off > 0 {
		return nil, nil
	}
	return []byte(s.id.String()), nil
}

func (s *session) Write(ctx fs.CtxI, off sp.Toffset, b []byte, f sp.Tfence) (sp.Tsize, *serr.Err) {
	if s.wctl == nil {
		return 0, serr.NewErr(serr.TErrNotSupported, nil)
	}
	return s.wctl(s.id, ctx, off, b, f)
}

func (s *session) Close(ctx fs.CtxI, m sp.Tmode) *serr.Err {
	db.DPrintf(db.CLONEDEV, "Close session ctl %v\n", s.id)
	return nil
}
