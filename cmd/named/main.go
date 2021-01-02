package main

import (
	"errors"
	// "log"

	"ulambda/fs"
	"ulambda/fssrv"
	np "ulambda/ninep"
)

type Named struct {
	done chan bool
	fs   *fs.Root
	srv  *fssrv.FsServer
}

func makeNamed() *Named {
	nd := &Named{}
	nd.srv = fssrv.MakeFsServer(nd, ":1111")
	nd.fs = fs.MakeRoot()
	nd.done = make(chan bool)
	return nd
}

func (nd *Named) Attach(conn *fssrv.FsConn, args np.Tattach, reply *np.Rattach) error {
	root := nd.fs.RootInode()
	conn.Fids[args.Fid] = root
	reply.Tag = args.Tag
	reply.Qid = *np.MakeQid(np.QTDIR, np.TQversion(root.Version), np.Tpath(root.Inum))
	return nil
}

func makeQids(inodes []*fs.Inode) []np.Tqid {
	var qids []np.Tqid
	for _, i := range inodes {
		qid := *np.MakeQid(np.QTDIR, np.TQversion(i.Version), np.Tpath(i.Inum))
		qids = append(qids, qid)
	}
	return qids
}

func (nd *Named) Walk(conn *fssrv.FsConn, args np.Twalk, reply *np.Rwalk) error {
	obj, ok := conn.Fids[args.Fid]
	if !ok {
		return errors.New("Unknown fid")
	}
	start := obj.(*fs.Inode)
	inodes, _, err := nd.fs.Walk(start.Data.(*fs.Dir), args.Path)
	if err != nil {
		return err
	}
	reply.Tag = args.Tag
	reply.Qids = makeQids(inodes)
	conn.Fids[args.NewFid] = inodes[len(inodes)-1]
	return nil
}

func (nd *Named) Create(conn *fssrv.FsConn, args np.Tcreate, reply *np.Rcreate) error {
	obj, ok := conn.Fids[args.Fid]
	if !ok {
		return errors.New("Unknown fid")
	}
	start := obj.(*fs.Inode)
	inode, err := nd.fs.Create(start, args.Name, args.Perm)
	if err != nil {
		return err
	}
	reply.Tag = args.Tag
	reply.Qid = *np.MakeQid(np.QTDIR, np.TQversion(inode.Version), np.Tpath(inode.Inum))
	return nil
}

func (nd *Named) Open(conn *fssrv.FsConn, args np.Topen, reply *np.Ropen) error {
	obj, ok := conn.Fids[args.Fid]
	if !ok {
		return errors.New("Unknown fid")
	}
	inode := obj.(*fs.Inode)
	reply.Tag = args.Tag
	reply.Qid = *np.MakeQid(np.QTDIR, np.TQversion(inode.Version), np.Tpath(inode.Inum))
	return nil
}

func (nd *Named) Clunk(conn *fssrv.FsConn, args np.Tclunk, reply *np.Rclunk) error {
	_, ok := conn.Fids[args.Fid]
	if !ok {
		return errors.New("Unknown fid")
	}
	delete(conn.Fids, args.Fid)
	return nil
}

func (nd *Named) Read(conn *fssrv.FsConn, args np.Tread, reply *np.Rread) error {
	obj, ok := conn.Fids[args.Fid]
	if !ok {
		return errors.New("Unknown fid")
	}
	inode := obj.(*fs.Inode)
	data, err := nd.fs.Read(inode, args.Count)
	if err != nil {
		return err
	}
	reply.Data = data
	return nil
}

func (nd *Named) Write(conn *fssrv.FsConn, args np.Twrite, reply *np.Rwrite) error {
	obj, ok := conn.Fids[args.Fid]
	if !ok {
		return errors.New("Unknown fid")
	}
	inode := obj.(*fs.Inode)
	n, err := nd.fs.Write(inode, args.Data)
	if err != nil {
		return err
	}
	reply.Count = n
	return nil
}

func main() {
	nd := makeNamed()
	<-nd.done
}
