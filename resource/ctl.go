package resource

import (
	"sigmaos/ctx"
	db "sigmaos/debug"
	"sigmaos/dir"
	"sigmaos/sessp"
    "sigmaos/serr"
	"sigmaos/fs"
	"sigmaos/inode"
	sp "sigmaos/sigmap"
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

func (ctl *CtlFile) Read(ctx fs.CtxI, off sp.Toffset, cnt sessp.Tsize, v sp.TQversion) ([]byte, *serr.Err) {
	return nil, serr.MkErr(serr.TErrNotSupported, "Read")
}

func (ctl *CtlFile) Write(ctx fs.CtxI, off sp.Toffset, b []byte, v sp.TQversion) (sessp.Tsize, *serr.Err) {
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
	return sessp.Tsize(len(b)), nil
}
