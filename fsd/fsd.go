package fsd

import (
	"errors"
	"log"
	"net"

	"ulambda/fs"
	"ulambda/fssrv"
	np "ulambda/ninep"
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

func (fsd *Fsd) Connect(conn net.Conn) fssrv.FsConn {
	clnt := makeFsConn(fsd.fs, conn)
	return clnt
}

func (fsc *FsConn) Version(args np.Tversion, reply *np.Rversion) error {
	log.Printf("Version %v\n", args)
	return errors.New("Not supported")
}

func (fsc *FsConn) Auth(args np.Tauth, reply *np.Rauth) error {
	log.Printf("Auth %v\n", args)
	return errors.New("Not supported")
}

func (fsc *FsConn) Attach(args np.Tattach, reply *np.Rattach) error {
	log.Printf("Attach %v from %v\n", args, fsc.conn.RemoteAddr())
	root := fsc.fs.RootInode()
	fsc.Fids[args.Fid] = root
	reply.Tag = args.Tag
	reply.Qid = np.MakeQid(np.QTDIR, np.TQversion(root.Version), np.Tpath(root.Inum))
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

func (fsc *FsConn) Walk(args np.Twalk, reply *np.Rwalk) error {
	log.Printf("Walk %v from %v\n", args, fsc.conn.RemoteAddr())
	start, ok := fsc.Fids[args.Fid]
	if !ok {
		return errors.New("Unknown fid")
	}
	inodes, _, err := fsc.fs.Walk(start.Data.(*fs.Dir), args.Path)
	if err != nil {
		return err
	}
	reply.Tag = args.Tag
	reply.Qids = makeQids(inodes)
	fsc.Fids[args.NewFid] = inodes[len(inodes)-1]
	return nil
}

func (fsc *FsConn) Create(args np.Tcreate, reply *np.Rcreate) error {
	log.Printf("Create %v from %v\n", args, fsc.conn.RemoteAddr())
	start, ok := fsc.Fids[args.Fid]
	if !ok {
		return errors.New("Unknown fid")
	}
	inode, err := fsc.fs.Create(start, args.Name, args.Perm)
	if err != nil {
		return err
	}
	reply.Tag = args.Tag
	reply.Qid = inode.Qid()
	return nil
}

func (fsc *FsConn) Mkdir(args np.Tmkdir, reply *np.Rmkdir) error {
	log.Printf("Mkdir %v from %v\n", args, fsc.conn.RemoteAddr())
	start, ok := fsc.Fids[args.Dfid]
	if !ok {
		return errors.New("Unknown fid")
	}
	inode, err := fsc.fs.Mkdir(start, args.Name)
	if err != nil {
		return err
	}
	reply.Tag = args.Tag
	reply.Qid = inode.Qid()
	return nil
}

func (fsc *FsConn) Symlink(args np.Tsymlink, reply *np.Rsymlink) error {
	log.Printf("Symlink %v from %v\n", args, fsc.conn.RemoteAddr())
	start, ok := fsc.Fids[args.Fid]
	if !ok {
		return errors.New("Unknown fid")
	}
	inode, err := fsc.fs.Symlink(start, args.Name, args.Symtgt)
	if err != nil {
		return err
	}
	reply.Tag = args.Tag
	reply.Qid = inode.Qid()
	return nil
}

func (fsc *FsConn) Readlink(args np.Treadlink, reply *np.Rreadlink) error {
	log.Printf("Readlink %v from %v\n", args, fsc.conn.RemoteAddr())
	inode, ok := fsc.Fids[args.Fid]
	if !ok {
		return errors.New("Unknown fid")
	}
	target, err := inode.Readlink()
	if err != nil {
		return err
	}
	reply.Tag = args.Tag
	reply.Target = target
	return nil
}

func (fsc *FsConn) Open(args np.Topen, reply *np.Ropen) error {
	log.Printf("Open %v from %v\n", args, fsc.conn.RemoteAddr())
	inode, ok := fsc.Fids[args.Fid]
	if !ok {
		return errors.New("Unknown fid")
	}
	reply.Tag = args.Tag
	reply.Qid = inode.Qid()
	return nil
}

func (fsc *FsConn) Clunk(args np.Tclunk, reply *np.Rclunk) error {
	log.Printf("Clunk %v\n", args)
	_, ok := fsc.Fids[args.Fid]
	if !ok {
		return errors.New("Unknown fid")
	}
	delete(fsc.Fids, args.Fid)
	return nil
}

func (fsc *FsConn) Flush(args np.Tflush, reply *np.Rflush) error {
	log.Printf("Flush %v\n", args)
	return errors.New("Not supported")
}

func (fsc *FsConn) Read(args np.Tread, reply *np.Rread) error {
	log.Printf("Read %v from %v\n", args, fsc.conn.RemoteAddr())
	inode, ok := fsc.Fids[args.Fid]
	if !ok {
		return errors.New("Unknown fid")
	}
	data, err := fsc.fs.Read(inode, args.Count)
	if err != nil {
		return err
	}
	reply.Data = data
	return nil
}

func (fsc *FsConn) Write(args np.Twrite, reply *np.Rwrite) error {
	log.Printf("Write %v from %v\n", args, fsc.conn.RemoteAddr())
	inode, ok := fsc.Fids[args.Fid]
	if !ok {
		return errors.New("Unknown fid")
	}
	n, err := fsc.fs.Write(inode, args.Data)
	if err != nil {
		return err
	}
	reply.Count = n
	return nil
}
