package fsd

import (
	"log"
	"net"

	"ulambda/fs"
	np "ulambda/ninep"
	"ulambda/npsrv"
)

type FsConn struct {
	fs   *fs.Root
	conn net.Conn
	Fids map[np.Tfid]*fs.Inode
}

func makeFsConn(root *fs.Root, conn net.Conn) *FsConn {
	clnt := &FsConn{root, conn, make(map[np.Tfid]*fs.Inode)}
	return clnt
}

type Fsd struct {
	fs *fs.Root
}

func MakeFsd() *Fsd {
	fsd := &Fsd{}
	fsd.fs = fs.MakeRoot()
	return fsd
}

func (fsd *Fsd) Root() *fs.Root {
	return fsd.fs
}

func (fsd *Fsd) Connect(conn net.Conn) npsrv.NpAPI {
	clnt := makeFsConn(fsd.fs, conn)
	return clnt
}

func (fsc *FsConn) Version(args np.Tversion, rets *np.Rversion) *np.Rerror {
	rets.Msize = args.Msize
	rets.Version = "9P2000"
	return nil
}

func (fsc *FsConn) Auth(args np.Tauth, rets *np.Rauth) *np.Rerror {
	return np.ErrUnknownMsg
}

func (fsc *FsConn) Attach(args np.Tattach, rets *np.Rattach) *np.Rerror {
	root := fsc.fs.RootInode()
	fsc.Fids[args.Fid] = root
	rets.Qid = np.MakeQid(np.QTDIR, np.TQversion(root.Version), np.Tpath(root.Inum))
	return nil
}

func makeQids(inodes []*fs.Inode) []np.Tqid {
	var qids []np.Tqid
	for _, i := range inodes {
		qid := i.Qid()
		qids = append(qids, qid)
	}
	return qids
}

func (fsc *FsConn) Walk(args np.Twalk, rets *np.Rwalk) *np.Rerror {
	start, ok := fsc.Fids[args.Fid]
	if !ok {
		return np.ErrUnknownfid
	}
	log.Printf("fsd.Walk %v from %v: dir %v\n", args, fsc.conn.RemoteAddr(), start)
	inodes, _, err := start.Walk(args.Wnames)
	if err != nil {
		return np.ErrNotfound
	}
	if len(inodes) == 0 { // clone args.Fid
		fsc.Fids[args.NewFid] = start
	} else {
		rets.Qids = makeQids(inodes)
		fsc.Fids[args.NewFid] = inodes[len(inodes)-1]
	}
	return nil
}

func (fsc *FsConn) Create(args np.Tcreate, rets *np.Rcreate) *np.Rerror {
	start, ok := fsc.Fids[args.Fid]
	if !ok {
		return np.ErrUnknownfid
	}
	log.Printf("fsd.Create %v from %v dir %v\n", args, fsc.conn.RemoteAddr(), start)
	inode, err := start.Create(fsc.fs, args.Perm, args.Name, []byte{})
	if err != nil {
		return np.ErrCreatenondir
	}
	fsc.Fids[args.Fid] = inode
	rets.Qid = inode.Qid()
	return nil
}

func (fsc *FsConn) Mkdir(args np.Tmkdir, rets *np.Rmkdir) *np.Rerror {
	start, ok := fsc.Fids[args.Dfid]
	if !ok {
		return np.ErrUnknownfid
	}
	log.Printf("fsd.Mkdir %v from %v dir %v\n", args, fsc.conn.RemoteAddr(), start)
	inode, err := fsc.fs.Mkdir(start, args.Name)
	if err != nil {
		return np.ErrCreatenondir
	}
	fsc.Fids[args.Dfid] = inode
	rets.Qid = inode.Qid()
	return nil
}

func (fsc *FsConn) Symlink(args np.Tsymlink, rets *np.Rsymlink) *np.Rerror {
	log.Printf("fsd.Symlink %v from %v\n", args, fsc.conn.RemoteAddr())
	start, ok := fsc.Fids[args.Fid]
	if !ok {
		return np.ErrUnknownfid
	}
	inode, err := start.Create(fsc.fs, np.DMSYMLINK, args.Name, fs.MakeSym(args.Symtgt))
	if err != nil {
		return np.ErrCreatenondir
	}
	rets.Qid = inode.Qid()
	return nil
}

func (fsc *FsConn) Pipe(args np.Tmkpipe, rets *np.Rmkpipe) *np.Rerror {
	start, ok := fsc.Fids[args.Dfid]
	if !ok {
		return np.ErrUnknownfid
	}
	inode, err := start.Create(fsc.fs, np.DMNAMEDPIPE, args.Name, fs.MakePipe())
	if err != nil {
		return np.ErrCreatenondir
	}
	rets.Qid = inode.Qid()
	return nil
}

func (fsc *FsConn) Readlink(args np.Treadlink, rets *np.Rreadlink) *np.Rerror {
	inode, ok := fsc.Fids[args.Fid]
	if !ok {
		return np.ErrUnknownfid
	}
	target, err := inode.Readlink()
	if err != nil {
		return np.ErrCreatenondir
	}
	rets.Target = target
	return nil
}

func (fsc *FsConn) Open(args np.Topen, rets *np.Ropen) *np.Rerror {
	inode, ok := fsc.Fids[args.Fid]
	if !ok {
		return np.ErrUnknownfid
	}
	rets.Qid = inode.Qid()
	return nil
}

func (fsc *FsConn) Clunk(args np.Tclunk, rets *np.Rclunk) *np.Rerror {
	_, ok := fsc.Fids[args.Fid]
	if !ok {
		return np.ErrUnknownfid
	}
	delete(fsc.Fids, args.Fid)
	return nil
}

func (fsc *FsConn) Flush(args np.Tflush, rets *np.Rflush) *np.Rerror {
	return nil
}

func (fsc *FsConn) Read(args np.Tread, rets *np.Rread) *np.Rerror {
	inode, ok := fsc.Fids[args.Fid]
	if !ok {
		return np.ErrUnknownfid
	}
	data, err := inode.Read(args.Offset, args.Count)
	if err != nil {
		return np.ErrBadcount
	}
	rets.Data = data
	return nil
}

func (fsc *FsConn) Write(args np.Twrite, rets *np.Rwrite) *np.Rerror {
	inode, ok := fsc.Fids[args.Fid]
	if !ok {
		return np.ErrUnknownfid
	}
	n, err := inode.Write(args.Offset, args.Data)
	if err != nil {
		return np.ErrBadcount
	}
	rets.Count = n
	return nil
}

func (fsc *FsConn) Stat(args np.Tstat, rets *np.Rstat) *np.Rerror {
	inode, ok := fsc.Fids[args.Fid]
	if !ok {
		return np.ErrUnknownfid
	}
	rets.Stat = *inode.Stat()
	return nil
}
