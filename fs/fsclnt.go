package fs

import (
	"errors"
	"log"
	"net"
	"net/rpc"
	"strings"

	"ulambda/fsrpc"
)

type Client struct {
	Fd       *fsrpc.Fd
	FdClient *rpc.Client
}

func (c *Client) connect() error {
	var err error
	c.FdClient, err = rpc.Dial("tcp", c.Fd.Addr)
	if err != nil {
		log.Fatal("dialing:", err)
	}
	return err
}

func (c *Client) makeCall(method string, req interface{}, reply interface{}) error {
	if c.FdClient == nil {
		err := c.connect()
		if err != nil {
			return err
		}
	}
	return c.FdClient.Call(method, req, reply)
}

type FsClient struct {
	root  *Client
	fds   map[int]*Client
	names map[string]*fsrpc.Fd
	first int
}

func MakeFsRoot() *fsrpc.Fd {
	tcpaddr, err := net.ResolveTCPAddr("tcp", "localhost"+":1111")
	if err != nil {
		log.Fatal("MakeFsRoot error", err)
	}
	return &fsrpc.Fd{tcpaddr.String()}
}

func MakeFsClient(root *fsrpc.Fd) *FsClient {
	fsc := &FsClient{
		&Client{root, nil},
		make(map[int]*Client),
		make(map[string]*fsrpc.Fd),
		0}
	return fsc
}

func (fsc *FsClient) findfd(Fd *fsrpc.Fd) int {
	fd := fsc.first
	fsc.fds[fd] = &Client{Fd, nil}
	fsc.first += 1
	return fd
}

func (fsc *FsClient) MkNod(path string, fs Fs) error {
	fd := register(fs, path == "/")
	fsc.names[path] = fd
	return nil
}

func (fsc *FsClient) Open(path string) (int, error) {
	if strings.HasPrefix(path, "/") { // remote lookup?
		args := fsrpc.OpenReq{path}
		var reply fsrpc.OpenReply
		err := fsc.root.makeCall("FsSrv.Open", args, &reply)
		if err == nil {
			fd := fsc.findfd(&reply.Fd)
			return fd, err
		} else {
			return -1, err
		}
	} else { // internal lookup
		if Fd, ok := fsc.names[path]; ok {
			fd := fsc.findfd(Fd)
			return fd, nil
		} else {
			return -1, errors.New("Open: non-existing pathname")
		}
	}
}

func (fsc *FsClient) Mount(fd int, path string) error {
	log.Printf("mount: %d %v\n", fd, path)
	// XXX should check if path starts with /
	if clnt, ok := fsc.fds[fd]; ok {
		args := fsrpc.MountReq{*clnt.Fd, path}
		var reply fsrpc.MountReply
		err := fsc.root.makeCall("FsSrv.Mount", args, &reply)
		return err
	} else {
		log.Printf("Mount: lookup error")
	}
	return errors.New("Mount: unknown fd")
}

func (fsc *FsClient) Write(fd int, buf []byte) (int, error) {
	if clnt, ok := fsc.fds[fd]; ok {
		log.Printf("Write at %v: %d\n", clnt.Fd, len(buf))
		args := fsrpc.WriteReq{buf}
		var reply fsrpc.WriteReply
		err := clnt.makeCall("FsSrv.Write", args, &reply)
		return reply.N, err
	} else {
		log.Printf("Write: lookup error %v", ok)
	}
	return -1, errors.New("Write: unknown fd")
}

func (fsc *FsClient) Read(fd int, n int) ([]byte, error) {
	if clnt, ok := fsc.fds[fd]; ok {
		log.Printf("Read at %v: %d\n", clnt.Fd, n)
		args := fsrpc.ReadReq{n}
		var reply fsrpc.ReadReply
		err := clnt.makeCall("FsSrv.Read", args, &reply)
		return reply.Buf, err
	} else {
		log.Printf("Read: lookup error %v", ok)
	}
	return nil, errors.New("Read: unknown fd")
}
