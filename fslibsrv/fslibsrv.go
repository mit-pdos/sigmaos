// Package fslibsrv allows caller to make a server and post their
// existence in the global name space. Servers plug in what a
// file/directory is by passing in their root directory, which is a
// concrete instance of the fs.Dir interface; for example, memfsd
// passes in an in-memory directory, fsux passes in a unix directory
// etc. This allows servers to implement their notions of
// directories/files, but they don't have to implement sigmaP, because
// fslibsrv provides that through sesssrv and protsrv.
package fslibsrv

import (
	db "sigmaos/debug"
	"sigmaos/ephemeralmap"
	"sigmaos/fs"
	"sigmaos/fsetcd"
	"sigmaos/path"
	"sigmaos/protsrv"
	"sigmaos/serr"
	"sigmaos/sesssrv"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
)

// Return the pathname for posting in a directory of a service
func mountPathName(pn string, mnt sp.Tmount) string {
	return pn + "/" + mnt.Address().IPPort()
}

func postMount(sesssrv *sesssrv.SessSrv, sc *sigmaclnt.SigmaClnt, pn string) (string, error) {
	mnt := sp.NewMountServer(sesssrv.MyAddr())
	db.DPrintf(db.BOOT, "Advertise %s at %v\n", pn, mnt)

	if path.EndSlash(pn) {
		dir, err := sc.IsDir(pn)
		if err != nil {
			return "", err
		}
		if !dir {
			return "", serr.NewErr(serr.TErrNotDir, pn)
		}
		pn = mountPathName(pn, mnt)
	}

	li, err := sc.LeaseClnt.AskLease(pn, fsetcd.LeaseTTL)
	if err != nil {
		return "", err
	}
	li.KeepExtending()

	if err := sc.MkMountFile(pn, mnt, li.Lease()); err != nil {
		return "", err
	}
	return pn, nil
}

func NewSrv(root fs.Dir, pn string, addr *sp.Taddr, sc *sigmaclnt.SigmaClnt, fencefs fs.Dir) (*sesssrv.SessSrv, string, error) {
	et := ephemeralmap.NewEphemeralMap()
	srv := sesssrv.NewSessSrv(sc.ProcEnv(), root, addr, protsrv.NewProtServer, nil, nil, et, fencefs)
	if len(pn) > 0 {
		if mpn, err := postMount(srv, sc, pn); err != nil {
			return nil, "", err
		} else {
			pn = mpn
		}
	}
	return srv, pn, nil
}
