package fs

import (
	"errors"
	"log"
	"net"
	"net/rpc"
	"path/filepath"
	"strconv"
	"strings"

	"ulambda/fsrpc"
	"ulambda/name"
)

const MAXFD = 20

type FsClient struct {
	fds   []*fsrpc.Ufd
	root  *fsrpc.Ufd
	clnts map[string]*rpc.Client
	srv   *name.Root
	first int
}

func MakeFsRoot() *fsrpc.Ufd {
	tcpaddr, err := net.ResolveTCPAddr("tcp", "localhost"+":1111")
	if err != nil {
		log.Fatal("MakeFsRoot error", err)
	}
	return &fsrpc.Ufd{tcpaddr.String(), 0}
}

func listenFsRoot() {
}

func MakeMyRoot(fs Fs, root bool) *fsrpc.Ufd {
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
	return &fsrpc.Ufd{addr.String(), 0}
}

func MakeFsClient(root *fsrpc.Ufd) *FsClient {
	fsc := &FsClient{}
	fsc.fds = make([]*fsrpc.Ufd, 0, MAXFD)
	fsc.root = MakeFsRoot()
	fsc.clnts = make(map[string]*rpc.Client)
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
func InitFsClient(root *fsrpc.Ufd, fds []string) (*FsClient, error) {
	fsc := MakeFsClient(root)
	for _, fd := range fds {
		parts := strings.Split(fd, "+")
		if len(parts) != 2 {
			return nil, errors.New("Bad fd")
		}
		i, err := strconv.Atoi(parts[1])
		if err != nil {
			return nil, errors.New("Bad fd")
		}
		fsc.findfd(&fsrpc.Ufd{parts[0], fsrpc.Fd(i)})
	}
	return fsc, nil
}

func (fsc *FsClient) makeCall(ufd *fsrpc.Ufd, method string, req interface{},
	reply interface{}) error {
	log.Printf("makeCall %s at %v %v\n", method, ufd.Addr, req)
	clnt, ok := fsc.clnts[ufd.Addr]
	if !ok {
		var err error
		clnt, err = rpc.Dial("tcp", ufd.Addr)
		if err != nil {
			return err
		}
		fsc.clnts[ufd.Addr] = clnt
	}
	return clnt.Call(method, req, reply)
}

func (fsc *FsClient) findfd(ufd *fsrpc.Ufd) int {
	fd := fsc.first
	fsc.fds = append(fsc.fds, ufd)
	fsc.first += 1
	return fd
}

func (fsc *FsClient) Lsof() []string {
	var fds []string
	for _, fd := range fsc.fds {
		if fd != nil {
			fds = append(fds, fd.Addr+"+"+strconv.Itoa(int(fd.Fd)))
		}
	}
	return fds

}

// XXX register empty interface and in fssvr check if method exists
func (fsc *FsClient) MkNod(path string, fs Fs) error {
	return nil
}

func (fsc *FsClient) walkOne(start *fsrpc.Ufd, path string) (*fsrpc.Ufd, string, error) {
	args := fsrpc.WalkReq{path}
	var reply fsrpc.WalkReply
	err := fsc.makeCall(start, "FsSrv.Walk", args, &reply)
	return &reply.Ufd, reply.Path, err
}

func (fsc *FsClient) Walk(path string) (*fsrpc.Ufd, error) {
	var start *fsrpc.Ufd
	var err error
	var rest string
	var ufd *fsrpc.Ufd

	if strings.HasPrefix(path, "/") { // remote lookup?
		start = fsc.root
	} else if fsc.srv != nil {
		start = fsc.srv.Myroot()
	} else {
		return nil, errors.New("Non-existing name")
	}
	p := path
	for {
		ufd, rest, err = fsc.walkOne(start, p)
		log.Printf("WalkOne %v -> %v rest %v %v\n", p, ufd, rest, err)
		if rest == "" || err != nil {
			break
		}
		start = ufd
		p = rest
	}
	return ufd, err
}

func (fsc *FsClient) Create(path string) (int, error) {
	ufd, err := fsc.Walk(filepath.Dir(path))
	args := fsrpc.CreateReq{filepath.Base(path)}
	var reply fsrpc.CreateReply
	err = fsc.makeCall(ufd, "FsSrv.Create", args, &reply)
	if err == nil {
		nufd := &fsrpc.Ufd{ufd.Addr, reply.Fd}
		fd := fsc.findfd(nufd)
		return fd, err
	} else {
		return -1, err
	}
}

func (fsc *FsClient) Open(path string) (int, error) {
	ufd, err := fsc.Walk(path)
	if err != nil {
		return -1, err
	}
	args := fsrpc.OpenReq{*ufd}
	var reply fsrpc.OpenReply
	err = fsc.makeCall(ufd, "FsSrv.Open", args, &reply)
	if err == nil {
		nufd := &fsrpc.Ufd{ufd.Addr, reply.Fd}
		fd := fsc.findfd(nufd)
		return fd, err
	} else {
		return -1, err
	}
}

func (fsc *FsClient) Mount(fd int, path string) error {
	// XXX should check if path starts with /
	Fd := fsc.fds[fd]
	if Fd != nil {
		args := fsrpc.MountReq{*Fd, path}
		var reply fsrpc.MountReply
		err := fsc.makeCall(fsc.root, "FsSrv.Mount", args, &reply)
		return err
	}
	return errors.New("Mount: unknown fd")
}

func (fsc *FsClient) Write(fd int, buf []byte) (int, error) {
	Fd := fsc.fds[fd]
	if Fd != nil {
		args := fsrpc.WriteReq{Fd.Fd, buf}
		var reply fsrpc.WriteReply
		err := fsc.makeCall(Fd, "FsSrv.Write", args, &reply)
		return reply.N, err
	}
	return -1, errors.New("Write: unknown fd")
}

func (fsc *FsClient) Read(fd int, n int) ([]byte, error) {
	Fd := fsc.fds[fd]
	if Fd != nil {
		args := fsrpc.ReadReq{Fd.Fd, n}
		var reply fsrpc.ReadReply
		err := fsc.makeCall(Fd, "FsSrv.Read", args, &reply)
		return reply.Buf, err
	}
	return nil, errors.New("Read: unknown fd")
}
