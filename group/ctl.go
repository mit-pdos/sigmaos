package group

import (
	"strings"

	"sigmaos/fs"
	"sigmaos/inode"
	sp "sigmaos/sigmap"
    "sigmaos/sessp"
)

type GroupCtl struct {
	fs.FsObj
	g *Group
}

func makeGroupCtl(ctx fs.CtxI, parent fs.Dir, kv *Group) fs.FsObj {
	i := inode.MakeInode(ctx, sp.DMDEVICE, parent)
	return &GroupCtl{i, kv}
}

func (c *GroupCtl) Write(ctx fs.CtxI, off sp.Toffset, b []byte, v sp.TQversion) (sessp.Tsize, *sessp.Err) {
	words := strings.Fields(string(b))
	if len(words) != 2 {
		return 0, sessp.MkErr(sessp.TErrInval, words)
	}
	err := c.g.op(words[0], words[1])
	return sessp.Tsize(len(b)), err
}

func (c *GroupCtl) Read(ctx fs.CtxI, off sp.Toffset, cnt sessp.Tsize, v sp.TQversion) ([]byte, *sessp.Err) {
	return nil, nil
}
