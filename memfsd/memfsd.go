package memfsd

import (
	"log"
	"net"

	"ulambda/memfs"
	np "ulambda/ninep"
	"ulambda/npsrv"
)

type Fid struct {
	path []string
	ino  *memfs.Inode
}

func makeFid(p []string, i *memfs.Inode) *Fid {
	return &Fid{p, i}
}

type NpConn struct {
	memfs *memfs.Root
	conn  net.Conn
	Fids  map[np.Tfid]*Fid
}

func makeNpConn(root *memfs.Root, conn net.Conn) *NpConn {
	npc := &NpConn{root, conn, make(map[np.Tfid]*Fid)}
	return npc
}

type Fsd struct {
	fs *memfs.Root
}

func MakeFsd() *Fsd {
	fsd := &Fsd{}
	fsd.fs = memfs.MakeRoot()
	return fsd
}

func (fsd *Fsd) Root() *memfs.Root {
	return fsd.fs
}

func (fsd *Fsd) Connect(conn net.Conn) npsrv.NpAPI {
	clnt := makeNpConn(fsd.fs, conn)
	return clnt
}

func (npc *NpConn) Version(args np.Tversion, rets *np.Rversion) *np.Rerror {
	rets.Msize = args.Msize
	rets.Version = "9P2000"
	return nil
}

func (npc *NpConn) Auth(args np.Tauth, rets *np.Rauth) *np.Rerror {
	return np.ErrUnknownMsg
}

func (npc *NpConn) Attach(args np.Tattach, rets *np.Rattach) *np.Rerror {
	root := npc.memfs.RootInode()
	npc.Fids[args.Fid] = makeFid([]string{}, root)
	rets.Qid = root.Qid()
	return nil
}

func makeQids(inodes []*memfs.Inode) []np.Tqid {
	var qids []np.Tqid
	for _, i := range inodes {
		qid := i.Qid()
		qids = append(qids, qid)
	}
	return qids
}

func (npc *NpConn) Walk(args np.Twalk, rets *np.Rwalk) *np.Rerror {
	fid, ok := npc.Fids[args.Fid]
	if !ok {
		return np.ErrUnknownfid
	}
	log.Printf("fsd.Walk %v from %v: dir %v\n", args, npc.conn.RemoteAddr(), fid)
	inodes, rest, err := fid.ino.Walk(args.Wnames)
	if err != nil {
		return &np.Rerror{err.Error()}
	}
	if len(inodes) == 0 { // clone args.Fid
		npc.Fids[args.NewFid] = makeFid(fid.path, fid.ino)
	} else {
		n := len(args.Wnames) - len(rest)
		p := append(fid.path, args.Wnames[:n]...)
		rets.Qids = makeQids(inodes)
		npc.Fids[args.NewFid] = makeFid(p, inodes[len(inodes)-1])
	}
	return nil
}

func (npc *NpConn) Open(args np.Topen, rets *np.Ropen) *np.Rerror {
	fid, ok := npc.Fids[args.Fid]
	if !ok {
		return np.ErrUnknownfid
	}
	rets.Qid = fid.ino.Qid()
	return nil
}

func (npc *NpConn) Create(args np.Tcreate, rets *np.Rcreate) *np.Rerror {
	fid, ok := npc.Fids[args.Fid]
	if !ok {
		return np.ErrUnknownfid
	}
	log.Printf("fsd.Create %v from %v dir %v\n", args, npc.conn.RemoteAddr(), fid)
	inode, err := fid.ino.Create(npc.memfs, args.Perm, args.Name)
	if err != nil {
		return &np.Rerror{err.Error()}
	}
	npc.Fids[args.Fid] = makeFid(append(fid.path, args.Name), inode)
	rets.Qid = inode.Qid()
	return nil
}

func (npc *NpConn) Clunk(args np.Tclunk, rets *np.Rclunk) *np.Rerror {
	_, ok := npc.Fids[args.Fid]
	if !ok {
		return np.ErrUnknownfid
	}
	delete(npc.Fids, args.Fid)
	return nil
}

func (npc *NpConn) Flush(args np.Tflush, rets *np.Rflush) *np.Rerror {
	return nil
}

func (npc *NpConn) Read(args np.Tread, rets *np.Rread) *np.Rerror {
	fid, ok := npc.Fids[args.Fid]
	if !ok {
		return np.ErrUnknownfid
	}
	data, err := fid.ino.Read(args.Offset, args.Count)
	if err != nil {
		return &np.Rerror{err.Error()}
	}
	rets.Data = data
	return nil
}

func (npc *NpConn) Write(args np.Twrite, rets *np.Rwrite) *np.Rerror {
	fid, ok := npc.Fids[args.Fid]
	if !ok {
		return np.ErrUnknownfid
	}
	n, err := fid.ino.Write(args.Offset, args.Data)
	if err != nil {
		return &np.Rerror{err.Error()}
	}
	rets.Count = n
	return nil
}

func (npc *NpConn) Remove(args np.Tremove, rets *np.Rremove) *np.Rerror {
	fid, ok := npc.Fids[args.Fid]
	if !ok {
		return np.ErrUnknownfid
	}
	err := fid.ino.Remove(npc.memfs, fid.path)
	if err != nil {
		return &np.Rerror{err.Error()}
	}
	delete(npc.Fids, args.Fid)
	return nil
}

func (npc *NpConn) Stat(args np.Tstat, rets *np.Rstat) *np.Rerror {
	fid, ok := npc.Fids[args.Fid]
	if !ok {
		return np.ErrUnknownfid
	}
	rets.Stat = *fid.ino.Stat()
	return nil
}

//
// Extension for ulambda
//

func (npc *NpConn) Pipe(args np.Tmkpipe, rets *np.Rmkpipe) *np.Rerror {
	fid, ok := npc.Fids[args.Dfid]
	if !ok {
		return np.ErrUnknownfid
	}
	inode, err := fid.ino.Create(npc.memfs, np.DMNAMEDPIPE, args.Name)
	if err != nil {
		return np.ErrCreatenondir
	}
	rets.Qid = inode.Qid()
	return nil
}
