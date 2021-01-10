package memfsd

import (
	"log"
	"net"
	"strings"
	"sync"

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
	mu    sync.Mutex // for Fids
	memfs *memfs.Root
	conn  net.Conn
	id    int
	Fids  map[np.Tfid]*Fid
}

func (npc *NpConn) lookup(fid np.Tfid) (*Fid, bool) {
	npc.mu.Lock()
	defer npc.mu.Unlock()
	f, ok := npc.Fids[fid]
	return f, ok
}

func makeNpConn(root *memfs.Root, conn net.Conn, id int) *NpConn {
	npc := &NpConn{}
	npc.memfs = root
	npc.conn = conn
	npc.id = id
	npc.Fids = make(map[np.Tfid]*Fid)
	return npc
}

type Fsd struct {
	fs     *memfs.Root
	nextId int
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
	fsd.nextId += 1
	clnt := makeNpConn(fsd.fs, conn, fsd.nextId)
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
	log.Printf("fsd.Walk %v from %v: dir %v\n", args, npc.conn.RemoteAddr(), fid)
	inodes, rest, err := fid.ino.Walk(npc.id, args.Wnames)
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
	rets.Qid = fid.ino.Qid()
	return nil
}

func (npc *NpConn) Create(args np.Tcreate, rets *np.Rcreate) *np.Rerror {
	fid, ok := npc.lookup(args.Fid)
	if !ok {
		return np.ErrUnknownfid
	}
	inode, err := fid.ino.Create(npc.id, npc.memfs, args.Perm, args.Name)
	if err != nil {
		return &np.Rerror{err.Error()}
	}
	npc.Fids[args.Fid] = makeFid(append(fid.path, args.Name), inode)
	rets.Qid = inode.Qid()
	return nil
}

func (npc *NpConn) Clunk(args np.Tclunk, rets *np.Rclunk) *np.Rerror {
	_, ok := npc.lookup(args.Fid)
	if !ok {
		return np.ErrUnknownfid
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
	err := root.Remove(npc.id, npc.memfs, fid.path)
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

func split(path string) []string {
	p := strings.Split(path, "/")
	return p
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
		// XXX cwd
		dst := split(args.Stat.Name)
		log.Print("dst path ", dst, fid.path)
		err := npc.memfs.Rename(npc.id, fid.path, dst)
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
	inode, err := fid.ino.Create(npc.id, npc.memfs, np.DMNAMEDPIPE, args.Name)
	if err != nil {
		return np.ErrCreatenondir
	}
	rets.Qid = inode.Qid()
	return nil
}
