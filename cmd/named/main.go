package main

import (
	// "log"

	"ulambda/fssrv"
	"ulambda/name"
	np "ulambda/ninep"
)

type Named struct {
	done chan bool
	name *name.Root
	srv  *fssrv.FsServer
}

func makeNamed() *Named {
	nd := &Named{}
	nd.srv = fssrv.MakeFsServer(nd, ":1111")
	nd.name = name.MakeRoot()
	nd.done = make(chan bool)
	return nd
}

func (nd *Named) Attach(conn *fssrv.FsConn, args np.Tattach, reply *np.Rattach) error {
	root := nd.name.RootInode()
	conn.Fids[args.Fid] = root
	reply.Tag = args.Tag
	reply.Qid = *np.MakeQid(np.QTDIR, np.TQversion(root.Version), np.Tpath(root.Inum))
	return nil
}

// func (nd *Named) Walk(start fid.Fid, path string) (*fid.Ufid, string, error) {
// 	nd.mu.Lock()
// 	defer nd.mu.Unlock()

// 	ufd, rest, err := nd.srv.Walk(start, path)
// 	return ufd, rest, err
// }

// func (nd *Named) Open(f fid.Fid) error {
// 	nd.mu.Lock()
// 	defer nd.mu.Unlock()

// 	_, err := nd.srv.OpenFid(f)
// 	return err
// }

// func (nd *Named) Symlink(f fid.Fid, src string, start *fid.Ufid, dst string) error {
// 	return errors.New("Unsupported")
// }

// func (nd *Named) Pipe(f fid.Fid, name string) error {
// 	return errors.New("Unsupported")
// }

// func (nd *Named) Create(f fid.Fid, t fid.IType, name string) (fid.Fid, error) {
// 	return fid.NullFid(), errors.New("Unsupported")
// }
//
//func (nd *Named) Remove(f fid.Fid, name string// ) error {
// 	return errors.New("Unsupported")
// }

// func (nd *Named) Write(f fid.Fid, buf []byte) (int, error) {
// 	return 0, errors.New("Unsupported")
// }

// func (nd *Named) Read(f fid.Fid, n int) ([]byte, error) {
// 	return nd.srv.Read(f, n)
// }

// func (nd *Named) Mount(uf *fid.Ufid, f fid.Fid, path string) error {
// 	nd.mu.Lock()
// 	defer nd.mu.Unlock()

// 	return nd.srv.Mount(uf, f, path)
// }

func main() {
	nd := makeNamed()
	<-nd.done
}
