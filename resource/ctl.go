package resource

import (
	"sigmaos/ctx"
	db "sigmaos/debug"
	"sigmaos/dir"
	"sigmaos/fs"
	"sigmaos/inode"
	np "sigmaos/sigmap"
)

type CtlFile struct {
	g ResourceGrantHandler
	r ResourceRequestHandler
	fs.Inode
}

func MakeCtlFile(g ResourceGrantHandler, r ResourceRequestHandler, parent fs.Dir, ctlFname string) {
	i := inode.MakeInode(nil, 0, parent)
	ctl := &CtlFile{g, r, i}
	err := dir.MkNod(ctx.MkCtx("", 0, nil), parent, ctlFname, ctl)
	if err != nil {
		db.DFatalf("Error MkNod: %v", err)
	}
}

func (ctl *CtlFile) Read(ctx fs.CtxI, off np.Toffset, cnt np.Tsize, v np.TQversion) ([]byte, *np.Err) {
	return nil, np.MkErr(np.TErrNotSupported, "Read")
}

func (ctl *CtlFile) Write(ctx fs.CtxI, off np.Toffset, b []byte, v np.TQversion) (np.Tsize, *np.Err) {
	msg := &ResourceMsg{}
	msg.Unmarshal(b)
	switch msg.MsgType {
	case Tgrant:
		ctl.g(msg)
	case Trequest:
		ctl.r(msg)
	default:
		db.DFatalf("Unknown message type")
	}
	return np.Tsize(len(b)), nil
}
