package fs

import (
	"errors"
	"log"
	"net"
	"net/rpc"
	"path/filepath"
	"strings"

	"ulambda/fsrpc"
)

type FsClient struct {
	root  *fsrpc.Ufd
	fds   map[int]*fsrpc.Ufd
	names map[string]*fsrpc.Ufd
	clnts map[string]*rpc.Client
	first int
}

func MakeFsRoot() *fsrpc.Ufd {
	tcpaddr, err := net.ResolveTCPAddr("tcp", "localhost"+":1111")
	if err != nil {
		log.Fatal("MakeFsRoot error", err)
	}
	return &fsrpc.Ufd{tcpaddr.String(), 0}
}

func MakeFsClient(root *fsrpc.Ufd) *FsClient {
	fsc := &FsClient{
		root,
		make(map[int]*fsrpc.Ufd),
		make(map[string]*fsrpc.Ufd),
		make(map[string]*rpc.Client),
		0}
	return fsc
}

func InitFsClient(root *fsrpc.Ufd, fds []string) *FsClient {
	fsc := MakeFsClient(root)
	for _, fd := range fds {
		fsc.findfd(&fsrpc.Ufd{fd, 0})
	}
	return fsc
}

func (fsc *FsClient) makeCall(ufd *fsrpc.Ufd, method string, req interface{},
	reply interface{}) error {
	log.Printf("makeCall %s at %v\n", method, ufd.Addr)
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
	fsc.fds[fd] = ufd
	fsc.first += 1
	return fd
}

func (fsc *FsClient) Lsof() []string {
	var fds []string
	for _, fd := range fsc.fds {
		if fd != nil {
			fds = append(fds, fd.Addr)
		}
	}
	return fds

}

func (fsc *FsClient) MkNod(path string, fs Fs) error {
	fd := register(fs, path == "/")
	fsc.names[path] = fd
	return nil
}

func (fsc *FsClient) Walk(start *fsrpc.Ufd, path string) (*fsrpc.Ufd, error) {
	args := fsrpc.WalkReq{path}
	var reply fsrpc.WalkReply
	err := fsc.makeCall(start, "FsSrv.Walk", args, &reply)
	return &reply.Ufd, err
}

func (fsc *FsClient) Create(path string) (int, error) {
	if strings.HasPrefix(path, "/") { // remote lookup?
		ufd, err := fsc.Walk(fsc.root, filepath.Dir(path))
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
	return -1, errors.New("Create: not supported")
}

func (fsc *FsClient) Open(path string) (int, error) {
	if strings.HasPrefix(path, "/") { // remote lookup?
		ufd, err := fsc.Walk(fsc.root, filepath.Dir(path))
		args := fsrpc.OpenReq{path}
		var reply fsrpc.OpenReply
		err = fsc.makeCall(ufd, "FsSrv.Open", args, &reply)
		if err == nil {
			nufd := &fsrpc.Ufd{ufd.Addr, reply.Fd}
			fd := fsc.findfd(nufd)
			return fd, err
		} else {
			return -1, err
		}
	} else { // internal lookup XXX should call server, locally
		if Fd, ok := fsc.names[path]; ok {
			fd := fsc.findfd(Fd)
			return fd, nil
		} else {
			return -1, errors.New("Open: non-existing pathname")
		}
	}
}

func (fsc *FsClient) Mount(fd int, path string) error {
	// XXX should check if path starts with /
	if Fd, ok := fsc.fds[fd]; ok {
		args := fsrpc.MountReq{*Fd, path}
		var reply fsrpc.MountReply
		err := fsc.makeCall(fsc.root, "FsSrv.Mount", args, &reply)
		return err
	}
	return errors.New("Mount: unknown fd")
}

func (fsc *FsClient) Write(fd int, buf []byte) (int, error) {
	if Fd, ok := fsc.fds[fd]; ok {
		args := fsrpc.WriteReq{Fd.Fd, buf}
		var reply fsrpc.WriteReply
		err := fsc.makeCall(Fd, "FsSrv.Write", args, &reply)
		return reply.N, err
	}
	return -1, errors.New("Write: unknown fd")
}

func (fsc *FsClient) Read(fd int, n int) ([]byte, error) {
	if Fd, ok := fsc.fds[fd]; ok {
		args := fsrpc.ReadReq{Fd.Fd, n}
		var reply fsrpc.ReadReply
		err := fsc.makeCall(Fd, "FsSrv.Read", args, &reply)
		return reply.Buf, err
	}
	return nil, errors.New("Read: unknown fd")
}
