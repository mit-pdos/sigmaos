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
	fid, err := s.fs.Create(req.Fid, req.Type, req.Name)
	if err == nil {
		reply.Fid = fid
	}
	return err
}

func (s *FsSrv) Remove(req fsrpc.RemoveReq, reply *fsrpc.RemoveReply) error {
	return s.fs.Remove(req.Fid, req.Name)
}

func (s *FsSrv) Open(req fsrpc.OpenReq, reply *fsrpc.OpenReply) error {
	err := s.fs.Open(req.Fid)
	reply.Err = err
	return err
}

func (s *FsSrv) Symlink(req fsrpc.SymlinkReq, reply *fsrpc.SymlinkReply) error {
	err := s.fs.Symlink(req.Fid, req.Src, &req.Start, req.Dst)
	reply.Err = err
	return err
}

func (s *FsSrv) Pipe(req fsrpc.PipeReq, reply *fsrpc.PipeReply) error {
	err := s.fs.Pipe(req.Fid, req.Name)
	reply.Err = err
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
