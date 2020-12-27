package fs

import (
	"ulambda/fsrpc"
)

type FsSrv struct {
	fs Fs
}

func (s *FsSrv) Walk(req fsrpc.WalkReq, reply *fsrpc.WalkReply) error {
	ufd, rest, err := s.fs.Walk(req.Fid, req.Name)
	if err == nil {
		reply.Path = rest
		reply.Ufid = *ufd
	}
	return err
}

func (s *FsSrv) Create(req fsrpc.CreateReq, reply *fsrpc.CreateReply) error {
	fid, err := s.fs.Create(req.Fid, req.Name)
	if err == nil {
		reply.Fid = fid
	}
	return err
}

func (s *FsSrv) Open(req fsrpc.OpenReq, reply *fsrpc.OpenReply) error {
	fd, err := s.fs.Open(req.Fid, req.Name)
	if err == nil {
		reply.Fid = fd
	}
	return err
}

func (s *FsSrv) Mount(req fsrpc.MountReq, reply *fsrpc.MountReply) error {
	err := s.fs.Mount(&req.Ufid, req.Fid, req.Name)
	reply.Err = err
	return err
}

func (s *FsSrv) Write(req fsrpc.WriteReq, reply *fsrpc.WriteReply) error {
	n, err := s.fs.Write(req.Fid, req.Buf)
	reply.N = n
	return err
}

func (s *FsSrv) Read(req fsrpc.ReadReq, reply *fsrpc.ReadReply) error {
	b, err := s.fs.Read(req.Fid, req.N)
	reply.Buf = b
	return err
}
