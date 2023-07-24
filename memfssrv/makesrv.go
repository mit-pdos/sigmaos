package memfssrv

import (
	"sigmaos/ctx"
	db "sigmaos/debug"
	"sigmaos/dir"
	"sigmaos/fslibsrv"
	"sigmaos/memfs"
	"sigmaos/portclnt"
	"sigmaos/proc"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
)

// Make an MemFs and advertise it at pn
func MakeMemFs(pn string, uname sp.Tuname) (*MemFs, error) {
	return MakeMemFsPort(pn, ":0", uname)
}

// Make an MemFs for a specific port and advertise it at pn
func MakeMemFsPort(pn, port string, uname sp.Tuname) (*MemFs, error) {
	sc, err := sigmaclnt.MkSigmaClnt(uname)
	if err != nil {
		return nil, err
	}
	db.DPrintf(db.PORT, "MakeMemFsPort %v %v\n", pn, port)
	fs, err := MakeMemFsPortClnt(pn, port, sc)
	return fs, err
}

// Make an MemFs for a specific port and client, and advertise it at
// pn
func MakeMemFsPortClnt(pn, port string, sc *sigmaclnt.SigmaClnt) (*MemFs, error) {
	root := dir.MkRootDir(ctx.MkCtxNull(), memfs.MakeInode, nil)
	srv, err := fslibsrv.MakeSrv(root, pn, port, sc)
	if err != nil {
		return nil, err
	}
	mfs := MakeMemFsSrv(sc.Uname(), pn, srv, sc)
	return mfs, nil
}

// Allocate server with public port and advertise it
func MakeMemFsPublic(pn string, uname sp.Tuname) (*MemFs, error) {
	sc, err := sigmaclnt.MkSigmaClnt(uname)
	if err != nil {
		return nil, err
	}
	pc, pi, err := portclnt.MkPortClntPort(sc.FsLib)
	if err != nil {
		return nil, err
	}
	// Make server without advertising mnt
	mfs, err := MakeMemFsPortClnt("", ":"+pi.Pb.RealmPort.String(), sc)
	if err != nil {
		return nil, err
	}
	mfs.pc = pc

	if err = pc.AdvertisePort(pn, pi, proc.GetNet(), mfs.MyAddr()); err != nil {
		return nil, err
	}
	return mfs, err
}
