package fs

import (
	"encoding/json"
	"errors"
	"log"
	"math/rand"
	"net"
	"net/rpc"
	"path/filepath"
	"strings"
	"time"

	"ulambda/fsrpc"
	"ulambda/name"
)

const MAXFD = 20

type FsClient struct {
	fds   []*fsrpc.Ufid
	root  *fsrpc.Ufid
	clnts map[string]*rpc.Client
	srv   *name.Root
	first int
	Proc  string
}

func MakeFsRoot() *fsrpc.Ufid {
	tcpaddr, err := net.ResolveTCPAddr("tcp", "localhost"+":1111")
	if err != nil {
		log.Fatal("MakeFsRoot error", err)
	}
	return &fsrpc.Ufid{tcpaddr.String(), fsrpc.RootFid()}
}

func listenFsRoot() {
}

func MakeMyRoot(fs Fs, root bool) *fsrpc.Ufid {
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
	return &fsrpc.Ufid{addr.String(), fsrpc.RootFid()}
}

func MakeFsClient(root *fsrpc.Ufid) *FsClient {
	fsc := &FsClient{}
	fsc.fds = make([]*fsrpc.Ufid, 0, MAXFD)
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
func InitFsClient(root *fsrpc.Ufid, fids []string) (*FsClient, error) {
	fsc := MakeFsClient(root)
	fsc.Proc = fids[0]
	log.Printf("InitFsClient %v\n", fsc.Proc)
	for _, fid := range fids[1:] {
		var ufid fsrpc.Ufid
		err := json.Unmarshal([]byte(fid), &ufid)
		if err != nil {
			return nil, errors.New("Bad fid")
		}
		fsc.findfd(&ufid)
	}
	return fsc, nil
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

func (fsc *FsClient) findfd(ufd *fsrpc.Ufid) int {
	fd := fsc.first
	fsc.fds = append(fsc.fds, ufd)
	fsc.first += 1
	return fd
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

func (fsc *FsClient) walkOne(start *fsrpc.Ufid, path string) (*fsrpc.Ufid, string, error) {
	args := fsrpc.WalkReq{start.Fid, path}
	var reply fsrpc.WalkReply
	err := fsc.makeCall(start.Addr, "FsSrv.Walk", args, &reply)
	return &reply.Ufid, reply.Path, err
}

func (fsc *FsClient) Walk(path string) (*fsrpc.Ufid, error) {
	var start *fsrpc.Ufid
	var err error
	var rest string
	var ufid *fsrpc.Ufid

	if strings.HasPrefix(path, "/") { // remote lookup?
		start = fsc.root
		path = strings.TrimLeft(path, "/")
	} else if fsc.srv != nil {
		start = fsc.srv.Myroot()
	} else {
		return nil, errors.New("Non-existing name")
	}
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

func (fsc *FsClient) Create(path string) (int, error) {
	ufid, err := fsc.Walk(filepath.Dir(path))
	if err != nil {
		return -1, err
	}
	args := fsrpc.CreateReq{ufid.Fid, filepath.Base(path)}
	var reply fsrpc.CreateReply
	err = fsc.makeCall(ufid.Addr, "FsSrv.Create", args, &reply)
	if err == nil {
		nufd := &fsrpc.Ufid{ufid.Addr, reply.Fid}
		fd := fsc.findfd(nufd)
		return fd, err
	} else {
		return -1, err
	}
}

func (fsc *FsClient) Open(path string) (int, error) {
	ufid, err := fsc.Walk(filepath.Dir(path))
	if err != nil {
		return -1, err
	}
	args := fsrpc.OpenReq{ufid.Fid, filepath.Base(path)}
	var reply fsrpc.OpenReply
	log.Printf("base %v\n", filepath.Base(path))
	err = fsc.makeCall(ufid.Addr, "FsSrv.Open", args, &reply)
	if err == nil {
		nufd := &fsrpc.Ufid{ufid.Addr, reply.Fid}
		fd := fsc.findfd(nufd)
		return fd, err
	} else {
		return -1, err
	}
}

func (fsc *FsClient) Mount(fd int, path string) error {
	ufid, err := fsc.Walk(filepath.Dir(path))
	if err != nil {
		return err
	}
	Fid := fsc.fds[fd]
	if Fid != nil {
		args := fsrpc.MountReq{*Fid, ufid.Fid, filepath.Base(path)}
		var reply fsrpc.MountReply
		err := fsc.makeCall(ufid.Addr, "FsSrv.Mount", args, &reply)
		return err
	}
	return errors.New("Mount: unknown fd")
}

func (fsc *FsClient) Write(fd int, buf []byte) (int, error) {
	Fid := fsc.fds[fd]
	if Fid != nil {
		args := fsrpc.WriteReq{Fid.Fid, buf}
		var reply fsrpc.WriteReply
		err := fsc.makeCall(Fid.Addr, "FsSrv.Write", args, &reply)
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
