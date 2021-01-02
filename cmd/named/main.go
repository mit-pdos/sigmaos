package main

import (
	"errors"
	// "log"

	"ulambda/fssrv"
	"ulambda/name"
	np "ulambda/ninep"
)

type Named struct {
	done chan bool
	name *name.Root
	srv  *fssrv.FsServer
}

func makeNamed() *Named {
	nd := &Named{}
	nd.srv = fssrv.MakeFsServer(nd, ":1111")
	nd.name = name.MakeRoot()
	nd.done = make(chan bool)
	return nd
}

func (nd *Named) Attach(conn *fssrv.FsConn, args np.Tattach, reply *np.Rattach) error {
	root := nd.name.RootInode()
	conn.Fids[args.Fid] = root
	reply.Tag = args.Tag
	reply.Qid = *np.MakeQid(np.QTDIR, np.TQversion(root.Version), np.Tpath(root.Inum))
	return nil
}

func makeQids(inodes []*name.Inode) []np.Tqid {
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
	start := obj.(*name.Inode)
	inodes, _, err := nd.name.Walk(start.Data.(*name.Dir), args.Path)
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
	start := obj.(*name.Inode)
	inode, err := nd.name.Create(start, args.Name, args.Perm)
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
	inode := obj.(*name.Inode)
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
	inode := obj.(*name.Inode)
	data, err := nd.name.Read(inode, args.Count)
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
	inode := obj.(*name.Inode)
	n, err := nd.name.Write(inode, args.Data)
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
