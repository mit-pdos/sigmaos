package fs

import (
	"ulambda/fsrpc"
)

type FsSrv struct {
	fs Fs
}

func (s *FsSrv) Open(req fsrpc.OpenReq, reply *fsrpc.OpenReply) error {
	fd, err := s.fs.Open(req.Name)
	reply.Fd = *fd
	return err
}

func (s *FsSrv) Mount(req fsrpc.MountReq, reply *fsrpc.MountReply) error {
	err := s.fs.Mount(&req.Fd, req.Name)
	reply.Err = err
	return err
}

func (s *FsSrv) Write(req fsrpc.WriteReq, reply *fsrpc.WriteReply) error {
	n, err := s.fs.Write(req.Buf)
	reply.N = n
	return err
}

func (s *FsSrv) Read(req fsrpc.ReadReq, reply *fsrpc.ReadReply) error {
	b, err := s.fs.Read(req.N)
	reply.Buf = b
	return err
}
