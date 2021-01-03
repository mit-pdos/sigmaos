package fsd

import (
	"errors"
	"net"
	// "log"

	"ulambda/fs"
	"ulambda/fssrv"
	np "ulambda/ninep"
)

type Client struct {
	fs   *fs.Root
	Fids map[np.Tfid]*fs.Inode
}

func makeClient(root *fs.Root) *Client {
	clnt := &Client{root, make(map[np.Tfid]*fs.Inode)}
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

func (fsd *Fsd) Connect(conn net.Conn) fssrv.FsClient {
	clnt := makeClient(fsd.fs)
	return clnt
}

func (clnt *Client) Attach(args np.Tattach, reply *np.Rattach) error {
	root := clnt.fs.RootInode()
	clnt.Fids[args.Fid] = root
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

func (clnt *Client) Walk(args np.Twalk, reply *np.Rwalk) error {
	start, ok := clnt.Fids[args.Fid]
	if !ok {
		return errors.New("Unknown fid")
	}
	inodes, _, err := clnt.fs.Walk(start.Data.(*fs.Dir), args.Path)
	if err != nil {
		return err
	}
	reply.Tag = args.Tag
	reply.Qids = makeQids(inodes)
	clnt.Fids[args.NewFid] = inodes[len(inodes)-1]
	return nil
}

func (clnt *Client) Create(args np.Tcreate, reply *np.Rcreate) error {
	start, ok := clnt.Fids[args.Fid]
	if !ok {
		return errors.New("Unknown fid")
	}
	inode, err := clnt.fs.Create(start, args.Name, args.Perm)
	if err != nil {
		return err
	}
	reply.Tag = args.Tag
	reply.Qid = inode.Qid()
	return nil
}

func (clnt *Client) Symlink(args np.Tsymlink, reply *np.Rsymlink) error {
	start, ok := clnt.Fids[args.Fid]
	if !ok {
		return errors.New("Unknown fid")
	}
	inode, err := clnt.fs.Symlink(start, args.Name, args.Symtgt)
	if err != nil {
		return err
	}
	reply.Tag = args.Tag
	reply.Qid = inode.Qid()
	return nil
}

func (clnt *Client) Readlink(args np.Treadlink, reply *np.Rreadlink) error {
	inode, ok := clnt.Fids[args.Fid]
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

func (clnt *Client) Open(args np.Topen, reply *np.Ropen) error {
	inode, ok := clnt.Fids[args.Fid]
	if !ok {
		return errors.New("Unknown fid")
	}
	reply.Tag = args.Tag
	reply.Qid = inode.Qid()
	return nil
}

func (clnt *Client) Clunk(args np.Tclunk, reply *np.Rclunk) error {
	_, ok := clnt.Fids[args.Fid]
	if !ok {
		return errors.New("Unknown fid")
	}
	delete(clnt.Fids, args.Fid)
	return nil
}

func (clnt *Client) Read(args np.Tread, reply *np.Rread) error {
	inode, ok := clnt.Fids[args.Fid]
	if !ok {
		return errors.New("Unknown fid")
	}
	data, err := clnt.fs.Read(inode, args.Count)
	if err != nil {
		return err
	}
	reply.Data = data
	return nil
}

func (clnt *Client) Write(args np.Twrite, reply *np.Rwrite) error {
	inode, ok := clnt.Fids[args.Fid]
	if !ok {
		return errors.New("Unknown fid")
	}
	n, err := clnt.fs.Write(inode, args.Data)
	if err != nil {
		return err
	}
	reply.Count = n
	return nil
}
