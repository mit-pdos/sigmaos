// Package sigmapsrv allows caller to make a sigmap server and post their
// existence in the global name space. Servers plug in what a
// file/directory is by passing in their root directory, which is a
// concrete instance of the fs.Dir interface; for example, memfsd
// passes in an in-memory directory, fsux passes in a unix directory
// etc. This allows servers to implement their notions of
// directories/files, but they don't have to implement sigmaP, because
// sigmapsrv provides that through sesssrv and protsrv.
package sigmapsrv

import (
	"sigmaos/auth"
	"sigmaos/ctx"
	db "sigmaos/debug"
	"sigmaos/dir"
	"sigmaos/fs"
	"sigmaos/fsetcd"
	"sigmaos/overlaydir"
	"sigmaos/path"
	"sigmaos/proc"
	"sigmaos/protsrv"
	"sigmaos/serr"
	"sigmaos/sessp"
	"sigmaos/sesssrv"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
	sps "sigmaos/sigmaprotsrv"
	"sigmaos/stats"
)

type SigmaPSrv struct {
	*protsrv.ProtSrvState
	*sesssrv.SessSrv
	srvmnt   *sp.Tmount
	dirunder fs.Dir
	dirover  *overlay.DirOverlay
	fencefs  fs.Dir
	stats    *stats.StatInfo
}

func NewSigmaPSrv(pe *proc.ProcEnv, root fs.Dir, as auth.AuthSrv, addr *sp.Taddr, fencefs fs.Dir) *SigmaPSrv {
	psrv := &SigmaPSrv{
		dirunder: root,
		dirover:  overlay.MkDirOverlay(root),
		fencefs:  fencefs,
	}
	psrv.stats = stats.NewStatsDev(psrv.dirover)
	psrv.ProtSrvState = protsrv.NewProtSrvState(as, psrv.stats)
	psrv.VersionTable().Insert(psrv.dirover.Path())
	psrv.dirover.Mount(sp.STATSD, psrv.stats)
	psrv.SessSrv = sesssrv.NewSessSrv(pe, addr, psrv.stats, psrv)
	return psrv
}

func NewSigmaPSrvPost(root fs.Dir, pn string, as auth.AuthSrv, addr *sp.Taddr, sc *sigmaclnt.SigmaClnt, fencefs fs.Dir) (*SigmaPSrv, string, error) {
	psrv := NewSigmaPSrv(sc.ProcEnv(), root, as, addr, fencefs)
	if len(pn) > 0 {
		if mpn, err := psrv.postMount(sc, pn); err != nil {
			return nil, "", err
		} else {
			pn = mpn
		}
	}
	return psrv, pn, nil
}

func (psrv *SigmaPSrv) NewSession(sessid sessp.Tsession) sps.Protsrv {
	return protsrv.NewProtServer(psrv.ProtSrvState, sessid, psrv.GetRootCtx)
}

func (psrv *SigmaPSrv) Root(path path.Path) (fs.Dir, path.Path) {
	d := psrv.dirunder
	if len(path) > 0 {
		o, err := psrv.dirover.Lookup(ctx.NewCtxNull(), path[0])
		if err == nil {
			return o.(fs.Dir), path[1:]
		}
	}
	return d, path
}

func (psrv *SigmaPSrv) Mount(name string, dir *dir.DirImpl) {
	dir.SetParent(psrv.dirover)
	psrv.dirover.Mount(name, dir)
}

func (psrv *SigmaPSrv) GetRootCtx(p *sp.Tprincipal, claims *auth.ProcClaims, aname string, sessid sessp.Tsession, clntid sp.TclntId) (fs.Dir, fs.CtxI) {
	return psrv.dirover, ctx.NewCtx(p, claims, sessid, clntid, psrv.CondTable(), psrv.fencefs)
}

func (psrv *SigmaPSrv) GetSigmaPSrvMount() *sp.Tmount {
	return psrv.srvmnt
}

func (psrv *SigmaPSrv) postMount(sc *sigmaclnt.SigmaClnt, pn string) (string, error) {
	mnt := psrv.GetMount()
	psrv.srvmnt = mnt
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

// Return the pathname for posting in a directory of a service
func mountPathName(pn string, mnt *sp.Tmount) string {
	return pn + "/" + mnt.Addrs()[0].IPPort()
}
