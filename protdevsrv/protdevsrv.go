package protdevsrv

import (
	"strconv"

	"sigmaos/ctx"
	db "sigmaos/debug"
	"sigmaos/dir"
	"sigmaos/fs"
	"sigmaos/fslibsrv"
	"sigmaos/inode"
	"sigmaos/memfs"
	np "sigmaos/ninep"
)

type stream struct {
	*inode.Inode
	fs.File
}

type streamCtl struct {
	*inode.Inode
	id string
}

func (sc *streamCtl) Read(ctx fs.CtxI, off np.Toffset, cnt np.Tsize, v np.TQversion) ([]byte, *np.Err) {
	if off > 0 {
		return nil, nil
	}
	return []byte(sc.id), nil
}

func (sc *streamCtl) Write(ctx fs.CtxI, off np.Toffset, b []byte, v np.TQversion) (np.Tsize, *np.Err) {
	return 0, np.MkErr(np.TErrNotSupported, nil)
}

func (sc *streamCtl) Close(ctx fs.CtxI, m np.Tmode) *np.Err {
	db.DPrintf("PROTDEVSRV", "Close ctl %v\n", sc.id)
	return nil
}

type MkStream func() (fs.File, *np.Err)

type Clone struct {
	fs.Inode
	mkStream MkStream
}

func makeClone(ctx fs.CtxI, parent fs.Dir, mkStream MkStream) fs.Inode {
	i := inode.MakeInode(ctx, np.DMDEVICE, parent)
	return &Clone{i, mkStream}
}

func (c *Clone) Open(ctx fs.CtxI, m np.Tmode) (fs.FsObj, *np.Err) {
	s := &streamCtl{}
	s.Inode = inode.MakeInode(nil, 0, nil)
	s.id = strconv.Itoa(int(s.Inode.Path()))

	db.DPrintf("PROTDEVSRV", "Open clone: create dir %v\n", s.id)

	// create directory for stream
	di := inode.MakeInode(nil, np.DMDIR, c.Parent())
	d := dir.MakeDir(di, memfs.MakeInode)
	err := dir.MkNod(ctx, c.Parent(), s.id, d)
	if err != nil {
		db.DFatalf("MkNod d %v err %v\n", d, err)
	}
	err = dir.MkNod(ctx, d, "ctl", s) // put ctl file into stream dir
	if err != nil {
		db.DFatalf("MkNod err %v\n", err)
	}

	// make data/stream file
	st := &stream{}
	st.Inode = inode.MakeInode(nil, 0, d)
	st.File, err = c.mkStream()
	if err != nil {
		db.DFatalf("mkStream err %v\n", err)
	}
	dir.MkNod(ctx, d, "data", st)

	return s, nil
}

func (c *Clone) Close(ctx fs.CtxI, m np.Tmode) *np.Err {
	db.DPrintf("PROTDEVSRV", "Close clone\n")
	return nil
}

func Run(fn string, mkStream MkStream) {
	mfs, _, _, error := fslibsrv.MakeMemFs(fn, "fsnet"+fn)
	if error != nil {
		db.DFatalf("protdevsrv.Run: %v\n", error)
	}
	err := dir.MkNod(ctx.MkCtx("", 0, nil), mfs.Root(), "clone", makeClone(nil, mfs.Root(), mkStream))
	if err != nil {
		db.DFatalf("MakeNod clone failed %v\n", err)
	}
	mfs.Serve()
	mfs.Done()
}
