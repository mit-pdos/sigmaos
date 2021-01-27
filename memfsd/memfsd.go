package memfsd

import (
	"net"
	"sync"

	"ulambda/memfs"
	np "ulambda/ninep"
	"ulambda/npsrv"
)

// XXX maybe overload func (npc *Npconn) Open ..
type Walker interface {
	Walk(string, []string) error
}

type Fid struct {
	path []string
	ino  *memfs.Inode
	mode np.Tmode
}

func makeFid(p []string, i *memfs.Inode) *Fid {
	return &Fid{p, i, 0}
}

type NpConn struct {
	mu    sync.Mutex // for Fids
	memfs *memfs.Root
	conn  net.Conn
	Fids  map[np.Tfid]*Fid
	uname string
	walk  Walker
}

func (npc *NpConn) lookup(fid np.Tfid) (*Fid, bool) {
	npc.mu.Lock()
	defer npc.mu.Unlock()
	f, ok := npc.Fids[fid]
	return f, ok
}

// XXX better plan for overload open/create/..??
func makeNpConn(root *memfs.Root, conn net.Conn, w Walker) *NpConn {
	npc := &NpConn{}
	npc.memfs = root
	npc.conn = conn
	npc.Fids = make(map[np.Tfid]*Fid)
	npc.walk = w
	return npc
}

type Fsd struct {
	fs   *memfs.Root
	walk Walker
}

func MakeFsd(fs *memfs.Root, w Walker) *Fsd {
	fsd := &Fsd{}
	fsd.fs = fs
	fsd.walk = w
	return fsd
}

func (fsd *Fsd) Root() *memfs.Root {
	return fsd.fs
}

func (fsd *Fsd) Connect(conn net.Conn) npsrv.NpAPI {
	clnt := makeNpConn(fsd.fs, conn, fsd.walk)
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
	npc.mu.Lock()
	npc.uname = args.Uname
	npc.Fids[args.Fid] = makeFid([]string{}, root)
	npc.mu.Unlock()
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
	fid, ok := npc.lookup(args.Fid)
	if !ok {
		return np.ErrUnknownfid
	}
	if npc.walk != nil {
		err := npc.walk.Walk(npc.uname, args.Wnames)
		if err != nil {
			return &np.Rerror{err.Error()}
		}
	}
	inodes, rest, err := fid.ino.Walk(npc.uname, args.Wnames)
	if err != nil {
		return np.ErrNotfound
	}
	if len(inodes) == 1 { // clone args.Fid
		npc.Fids[args.NewFid] = makeFid(fid.path, fid.ino)
	} else {
		n := len(args.Wnames) - len(rest)
		p := append(fid.path, args.Wnames[:n]...)
		rets.Qids = makeQids(inodes[1:])
		npc.Fids[args.NewFid] = makeFid(p, inodes[len(inodes)-1])
	}
	return nil
}

func (npc *NpConn) Open(args np.Topen, rets *np.Ropen) *np.Rerror {
	fid, ok := npc.lookup(args.Fid)
	if !ok {
		return np.ErrUnknownfid
	}
	err := fid.ino.Open(args.Mode)
	if err != nil {
		return &np.Rerror{err.Error()}
	}
	fid.mode = args.Mode
	rets.Qid = fid.ino.Qid()
	return nil
}

func (npc *NpConn) Create(args np.Tcreate, rets *np.Rcreate) *np.Rerror {
	fid, ok := npc.lookup(args.Fid)
	if !ok {
		return np.ErrUnknownfid
	}
	names := []string{args.Name}
	if npc.walk != nil {
		err := npc.walk.Walk(npc.uname, names)
		if err != nil {
			return &np.Rerror{err.Error()}
		}
	}
	inode, err := fid.ino.Create(npc.uname, npc.memfs, args.Perm, names[0])
	if err != nil {
		return &np.Rerror{err.Error()}
	}
	npc.Fids[args.Fid] = makeFid(append(fid.path, args.Name), inode)
	rets.Qid = inode.Qid()
	return nil
}

func (npc *NpConn) Clunk(args np.Tclunk, rets *np.Rclunk) *np.Rerror {
	fid, ok := npc.lookup(args.Fid)
	if !ok {
		return np.ErrUnknownfid
	}
	err := fid.ino.Close(fid.mode)
	if err != nil {
		return &np.Rerror{err.Error()}
	}
	npc.mu.Lock()
	delete(npc.Fids, args.Fid)
	npc.mu.Unlock()
	return nil
}

func (npc *NpConn) Flush(args np.Tflush, rets *np.Rflush) *np.Rerror {
	return nil
}

func (npc *NpConn) Read(args np.Tread, rets *np.Rread) *np.Rerror {
	fid, ok := npc.lookup(args.Fid)
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
	fid, ok := npc.lookup(args.Fid)
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

// XXX cwd lookups
func (npc *NpConn) Remove(args np.Tremove, rets *np.Rremove) *np.Rerror {
	root := npc.memfs.RootInode()
	fid, ok := npc.lookup(args.Fid)
	if !ok {
		return np.ErrUnknownfid
	}
	err := root.Remove(npc.uname, npc.memfs, fid.path)
	if err != nil {
		return &np.Rerror{err.Error()}
	}
	delete(npc.Fids, args.Fid)
	return nil
}

func (npc *NpConn) Stat(args np.Tstat, rets *np.Rstat) *np.Rerror {
	fid, ok := npc.lookup(args.Fid)
	if !ok {
		return np.ErrUnknownfid
	}
	rets.Stat = *fid.ino.Stat()
	return nil
}

func (npc *NpConn) Wstat(args np.Twstat, rets *np.Rwstat) *np.Rerror {
	fid, ok := npc.lookup(args.Fid)
	if !ok {
		return np.ErrUnknownfid
	}
	if args.Stat.Name != "" {
		// XXX No rename across 9p servers
		if args.Stat.Name[0] == '/' {
			return np.ErrUnknownMsg
		}
		// XXX renames within same dir
		dst := make([]string, len(fid.path))
		copy(dst, fid.path)
		dst = append(dst[:len(dst)-1], np.Split(args.Stat.Name)...)
		err := npc.memfs.Rename(npc.uname, fid.path, dst)
		if err != nil {
			return &np.Rerror{err.Error()}
		}
		fid.path = dst
	}
	return nil
}

//
// Extension for ulambda
//

func (npc *NpConn) Pipe(args np.Tmkpipe, rets *np.Rmkpipe) *np.Rerror {
	fid, ok := npc.lookup(args.Dfid)
	if !ok {
		return np.ErrUnknownfid
	}
	inode, err := fid.ino.Create(npc.uname, npc.memfs, np.DMNAMEDPIPE, args.Name)
	if err != nil {
		return np.ErrCreatenondir
	}
	rets.Qid = inode.Qid()
	return nil
}
