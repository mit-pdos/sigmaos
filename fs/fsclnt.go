package fs

import (
	"encoding/json"
	"errors"
	"log"
	"math/rand"
	"net"
	"net/rpc"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"ulambda/fid"
	"ulambda/fsrpc"
	"ulambda/name"
)

const (
	Stdin  = 0
	Stdout = 1
	// Stderr = 2
)

const MAXFD = 20

type FsClient struct {
	fds   []*fid.Ufid
	root  *fid.Ufid
	clnts map[string]*rpc.Client
	srv   *name.Root
	Proc  string
}

func MakeFsRoot() *fid.Ufid {
	tcpaddr, err := net.ResolveTCPAddr("tcp", "localhost"+":1111")
	if err != nil {
		log.Fatal("MakeFsRoot error", err)
	}
	return &fid.Ufid{tcpaddr.String(), fid.RootFid()}
}

func listenFsRoot() {
}

func MakeMyRoot(fs Fs, root bool) *fid.Ufid {
	var l net.Listener
	var err error
	if root {
		l, err = net.Listen("tcp", ":1111")
	} else {
		l, err = net.Listen("tcp", ":0")
	}
	if err != nil {
		log.Fatal("Listen error:", err)
	}
	addr := l.Addr()
	log.Printf("myaddr %v\n", addr)
	register(l, fs)
	return &fid.Ufid{addr.String(), fid.RootFid()}
}

func MakeFsClient(root *fid.Ufid) *FsClient {
	fsc := &FsClient{}
	fsc.fds = make([]*fid.Ufid, 0, MAXFD)
	fsc.root = MakeFsRoot()
	fsc.clnts = make(map[string]*rpc.Client)
	rand.Seed(time.Now().UnixNano())
	return fsc
}

func MakeFsServer(fs Fs, root bool) *name.Root {
	myroot := MakeMyRoot(fs, root)
	return name.MakeRoot(myroot)
}

func MakeFs(fs Fs, root bool) (*FsClient, *name.Root) {
	fsc := MakeFsClient(MakeFsRoot())
	fsc.srv = MakeFsServer(fs, root)
	return fsc, fsc.srv
}

// XXX use gob?
func InitFsClient(root *fid.Ufid, args []string) (*FsClient, []string, error) {
	log.Printf("InitFsClient: %v\n", args)
	if len(args) < 2 {
		return nil, nil, errors.New("Missing len and program")
	}
	n, err := strconv.Atoi(args[0])
	if err != nil {
		return nil, nil, errors.New("Bad arg len")
	}
	if n < 1 {
		return nil, nil, errors.New("Missing program")
	}
	a := args[1 : n+1] // skip len and +1 for program name
	fids := args[n+1:]
	fsc := MakeFsClient(root)
	fsc.Proc = a[0]
	log.Printf("Args %v fids %v\n", a, fids)
	for _, f := range fids {
		var uf fid.Ufid
		err := json.Unmarshal([]byte(f), &uf)
		if err != nil {
			return nil, nil, errors.New("Bad fid")
		}
		fsc.findfd(&uf)
	}
	return fsc, a, nil
}

func (fsc *FsClient) makeCall(addr string, method string, req interface{},
	reply interface{}) error {
	log.Printf("makeCall %s at %v %v\n", method, addr, req)
	clnt, ok := fsc.clnts[addr]
	if !ok {
		var err error
		clnt, err = rpc.Dial("tcp", addr)
		if err != nil {
			return err
		}
		fsc.clnts[addr] = clnt
	}
	return clnt.Call(method, req, reply)
}

func (fsc *FsClient) findfd(nufd *fid.Ufid) int {
	for fd, ufd := range fsc.fds {
		if ufd == nil {
			fsc.fds[fd] = nufd
			return fd
		}
	}
	// no free one
	fsc.fds = append(fsc.fds, nufd)
	return len(fsc.fds) - 1
}

func (fsc *FsClient) Close(fd int) error {
	if fsc.fds[fd] == nil {
		return errors.New("Close: fd isn't open")
	}
	fsc.fds[fd] = nil
	return nil
}

func (fsc *FsClient) Lsof() []string {
	var fids []string
	for _, fid := range fsc.fds {
		if fid != nil {
			b, err := json.Marshal(fid)
			if err != nil {
				log.Fatal("Marshall error:", err)
			}
			fids = append(fids, string(b))
		}
	}
	return fids
}

func (fsc *FsClient) walkOne(start *fid.Ufid, path string) (*fid.Ufid, string, error) {
	args := fsrpc.NameReq{start.Fid, path}
	var reply fsrpc.WalkReply
	err := fsc.makeCall(start.Addr, "FsSrv.Walk", args, &reply)
	return &reply.Ufid, reply.Path, err
}

func (fsc *FsClient) walkAt(ufid *fid.Ufid, path string) (*fid.Ufid, error) {
	var err error
	var rest string
	start := ufid
	p := path
	for {

		ufid, rest, err = fsc.walkOne(start, p)
		log.Printf("WalkOne %v -> %v rest %v %v\n", p, ufid, rest, err)
		if rest == "" || err != nil {
			break
		}
		start = ufid
		p = rest
	}
	return ufid, err
}

func (fsc *FsClient) Walk(path string) (*fid.Ufid, error) {
	if strings.HasPrefix(path, "/") { // remote lookup?
		return fsc.walkAt(fsc.root, strings.TrimLeft(path, "/"))
	} else if fsc.srv != nil {
		return fsc.walkAt(fsc.srv.Myroot(), path)
	} else {
		return nil, errors.New("Non-existing name")
	}
}

func (fsc *FsClient) createat(uf *fid.Ufid, path string, t fid.IType) (int, error) {
	args := fsrpc.CreateReq{uf.Fid, filepath.Base(path), t}
	var reply fsrpc.FidReply
	err := fsc.makeCall(uf.Addr, "FsSrv.Create", args, &reply)
	if err == nil {
		nufd := &fid.Ufid{uf.Addr, reply.Fid}
		fd := fsc.findfd(nufd)
		return fd, err
	} else {
		return -1, err
	}
}

// XXX use walkAt to resolve path?
func (fsc *FsClient) CreateAt(fd int, path string) (int, error) {
	ufid := fsc.fds[fd]
	return fsc.createat(ufid, filepath.Base(path), fid.FileT)
}

func (fsc *FsClient) Create(path string) (int, error) {
	ufid, err := fsc.Walk(filepath.Dir(path))
	if err != nil {
		return -1, err
	}
	return fsc.createat(ufid, filepath.Base(path), fid.FileT)
}

func (fsc *FsClient) remove(uf *fid.Ufid, name string) error {
	args := fsrpc.NameReq{uf.Fid, name}
	return fsc.makeCall(uf.Addr, "FsSrv.Remove", args, nil)
}

func (fsc *FsClient) Remove(path string) error {
	ufid, err := fsc.Walk(filepath.Dir(path))
	if err != nil {
		return err
	}
	return fsc.remove(ufid, filepath.Base(path))
}

func (fsc *FsClient) MkDir(path string) (int, error) {
	ufid, err := fsc.Walk(filepath.Dir(path))
	if err != nil {
		return -1, err
	}
	fd, err := fsc.createat(ufid, filepath.Base(path), fid.DirT)
	return fd, err
}

func (fsc *FsClient) RmDir(path string) error {
	ufid, err := fsc.Walk(filepath.Dir(path))
	if err != nil {
		return err
	}
	return fsc.remove(ufid, filepath.Base(path))
}

func (fsc *FsClient) open(ufid *fid.Ufid) (int, error) {
	args := fsrpc.FidReq{ufid.Fid}
	var reply fsrpc.EmptyReply
	err := fsc.makeCall(ufid.Addr, "FsSrv.Open", args, &reply)
	if err == nil {
		fd := fsc.findfd(ufid)
		return fd, err
	} else {
		return -1, err
	}
}

func (fsc *FsClient) OpenAt(fd int, path string) (int, error) {
	ufid, err := fsc.walkAt(fsc.fds[fd], path)
	if err != nil {
		return -1, err
	}
	return fsc.open(ufid)
}

func (fsc *FsClient) Open(path string) (int, error) {
	ufid, err := fsc.Walk(path)
	if err != nil {
		return -1, err
	}
	return fsc.open(ufid)
}

func (fsc *FsClient) symlink(ufid *fid.Ufid, src string, dst string) error {
	var start *fid.Ufid
	if strings.HasPrefix(dst, "/") { // remote lookup?
		start = fsc.root
		dst = strings.TrimLeft(dst, "/")
	} else if fsc.srv != nil {
		start = fsc.srv.Myroot()
	} else {
		return errors.New("Non-existing dst name")
	}
	args := fsrpc.SymlinkReq{ufid.Fid, filepath.Base(src), *start, dst}
	var reply fsrpc.SymlinkReply
	return fsc.makeCall(ufid.Addr, "FsSrv.Symlink", args, &reply)
}

func (fsc *FsClient) SymlinkAt(fd int, src string, dst string) error {
	ufid := fsc.fds[fd]
	return fsc.symlink(ufid, filepath.Base(src), dst)
}

func (fsc *FsClient) Symlink(src string, dst string) error {
	ufid, err := fsc.Walk(filepath.Dir(src))
	if err != nil {
		return err
	}
	return fsc.symlink(ufid, filepath.Base(src), dst)
}

func (fsc *FsClient) Pipe(path string) error {
	ufid, err := fsc.Walk(filepath.Dir(path))
	if err != nil {
		return err
	}
	args := fsrpc.NameReq{ufid.Fid, filepath.Base(path)}
	var reply fsrpc.EmptyReply
	return fsc.makeCall(ufid.Addr, "FsSrv.Pipe", args, &reply)
}

func (fsc *FsClient) Mount(fd int, path string) error {
	ufid, err := fsc.Walk(filepath.Dir(path))
	if err != nil {
		return err
	}
	Fid := fsc.fds[fd]
	if Fid != nil {
		args := fsrpc.MountReq{*Fid, ufid.Fid, filepath.Base(path)}
		var reply fsrpc.EmptyReply
		return fsc.makeCall(ufid.Addr, "FsSrv.Mount", args, &reply)
	}
	return errors.New("Mount: unknown fd")
}

func (fsc *FsClient) Write(fd int, buf []byte) (int, error) {
	ufid := fsc.fds[fd]
	if ufid != nil {
		args := fsrpc.WriteReq{ufid.Fid, buf}
		var reply fsrpc.WriteReply
		err := fsc.makeCall(ufid.Addr, "FsSrv.Write", args, &reply)
		return reply.N, err
	}
	return -1, errors.New("Write: unknown fd")
}

func (fsc *FsClient) Read(fd int, n int) ([]byte, error) {
	Fid := fsc.fds[fd]
	if Fid != nil {
		args := fsrpc.ReadReq{Fid.Fid, n}
		var reply fsrpc.ReadReply
		err := fsc.makeCall(Fid.Addr, "FsSrv.Read", args, &reply)
		return reply.Buf, err
	}
	return nil, errors.New("Read: unknown fd")
}
