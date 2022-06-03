package realm

import (
	db "ulambda/debug"
	"ulambda/fs"
	"ulambda/inode"
	np "ulambda/ninep"
	"ulambda/resource"
)

/*
 * Diagram of the realm fs structure:
 * /
 * |- realmmgr // Realm manager fs.
 * |  |- free-machineds // Control file for free machineds to ask for a realm assignment.
 * |  |- realm-create // Control file to create a realm.
 * |  |- realm-destroy // Control file to destroy a realm.
 * |
 * |- realm-fences
 * |  |- realm-1-fence // Fence/lock used to ensure mutual exclusion when adding/removing machineds to/from a realm.
 * |  |- ...
 * |
 * |- realm-config // Stores a config file for each realm.
 * |  |- realm-1-config
 * |  |- ...
 * |
 * |- machined-config // Stores a config file for each machined.
 * |  |- machined-5-config
 * |  |- ...
 * |
 * |- realm-nameds // Stores symlinks to each realm's named replicas.
 * |  |- realm-1 -> [127.0.0.1:1234,127.0.0.1:4567,...] (named replicas)
 * |  |- ...
 * |  |- ...
 */

type CtlFile struct {
	g resource.ResourceGrantHandler
	r resource.ResourceRequestHandler
	fs.Inode
}

func makeCtlFile(g resource.ResourceGrantHandler, r resource.ResourceRequestHandler, ctx fs.CtxI, parent fs.Dir) *CtlFile {
	i := inode.MakeInode(ctx, 0, parent)
	return &CtlFile{g, r, i}
}

func (ctl *CtlFile) Read(ctx fs.CtxI, off np.Toffset, cnt np.Tsize, v np.TQversion) ([]byte, *np.Err) {
	return nil, np.MkErr(np.TErrNotSupported, "Read")
}

func (ctl *CtlFile) Write(ctx fs.CtxI, off np.Toffset, b []byte, v np.TQversion) (np.Tsize, *np.Err) {
	msg := &resource.ResourceMsg{}
	msg.Unmarshal(b)
	switch msg.MsgType {
	case resource.Tgrant:
		ctl.g(msg)
	case resource.Trequest:
		ctl.r(msg)
	default:
		db.DFatalf("Unknown message type")
	}
	return np.Tsize(len(b)), nil
}
