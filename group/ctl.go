package group

import (
	"strings"

	"sigmaos/fs"
	"sigmaos/inode"
	np "sigmaos/sigmap"
    "sigmaos/fcall"
)

type GroupCtl struct {
	fs.FsObj
	g *Group
}

func makeGroupCtl(ctx fs.CtxI, parent fs.Dir, kv *Group) fs.FsObj {
	i := inode.MakeInode(ctx, np.DMDEVICE, parent)
	return &GroupCtl{i, kv}
}

func (c *GroupCtl) Write(ctx fs.CtxI, off np.Toffset, b []byte, v np.TQversion) (np.Tsize, *fcall.Err) {
	words := strings.Fields(string(b))
	if len(words) != 2 {
		return 0, fcall.MkErr(fcall.TErrInval, words)
	}
	err := c.g.op(words[0], words[1])
	return np.Tsize(len(b)), err
}

func (c *GroupCtl) Read(ctx fs.CtxI, off np.Toffset, cnt np.Tsize, v np.TQversion) ([]byte, *fcall.Err) {
	return nil, nil
}
