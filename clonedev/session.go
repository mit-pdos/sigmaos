package clonedev

import (
	db "sigmaos/debug"
	"sigmaos/fs"
	"sigmaos/inode"
	np "sigmaos/sigmap"
)

type session struct {
	*inode.Inode
	id np.Tsession
}

func (s *session) Read(ctx fs.CtxI, off np.Toffset, cnt np.Tsize, v np.TQversion) ([]byte, *np.Err) {
	if off > 0 {
		return nil, nil
	}
	return []byte(s.id.String()), nil
}

func (s *session) Write(ctx fs.CtxI, off np.Toffset, b []byte, v np.TQversion) (np.Tsize, *np.Err) {
	return 0, np.MkErr(np.TErrNotSupported, nil)
}

func (s *session) Close(ctx fs.CtxI, m np.Tmode) *np.Err {
	db.DPrintf("CLONEDEV", "Close session ctl %v\n", s.id)
	return nil
}
