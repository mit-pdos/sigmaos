package clonedev

import (
	db "sigmaos/debug"
	"sigmaos/fs"
	"sigmaos/inode"
	sp "sigmaos/sigmap"
    "sigmaos/fcall"
)

type session struct {
	*inode.Inode
	id fcall.Tsession
}

func (s *session) Read(ctx fs.CtxI, off sp.Toffset, cnt sp.Tsize, v sp.TQversion) ([]byte, *fcall.Err) {
	if off > 0 {
		return nil, nil
	}
	return []byte(s.id.String()), nil
}

func (s *session) Write(ctx fs.CtxI, off sp.Toffset, b []byte, v sp.TQversion) (sp.Tsize, *fcall.Err) {
	return 0, fcall.MkErr(fcall.TErrNotSupported, nil)
}

func (s *session) Close(ctx fs.CtxI, m sp.Tmode) *fcall.Err {
	db.DPrintf("CLONEDEV", "Close session ctl %v\n", s.id)
	return nil
}
