package realm

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

import (
	"ulambda/fs"
	"ulambda/inode"
	np "ulambda/ninep"
)

type CtlFile struct {
	queue chan string
	fs.Inode
}

func makeCtlFile(queue chan string, ctx fs.CtxI, parent fs.Dir) *CtlFile {
	i := inode.MakeInode(ctx, 0, parent)
	return &CtlFile{queue, i}
}

func (ctl *CtlFile) Read(ctx fs.CtxI, off np.Toffset, cnt np.Tsize, v np.TQversion) ([]byte, *np.Err) {
	return nil, np.MkErr(np.TErrNotSupported, "Read")
}

func (ctl *CtlFile) Write(ctx fs.CtxI, off np.Toffset, b []byte, v np.TQversion) (np.Tsize, *np.Err) {
	ctl.queue <- string(b)
	return np.Tsize(len(b)), nil
}
