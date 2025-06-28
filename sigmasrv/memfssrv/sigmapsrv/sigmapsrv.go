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
	"time"

	"sigmaos/api/fs"
	sps "sigmaos/api/spprotsrv"
	"sigmaos/ctx"
	db "sigmaos/debug"
	dialproxyclnt "sigmaos/dialproxy/clnt"
	"sigmaos/namesrv/fsetcd"
	"sigmaos/path"
	"sigmaos/proc"
	"sigmaos/serr"
	sessp "sigmaos/session/proto"
	sesssrv "sigmaos/session/srv"
	"sigmaos/sigmaclnt"
	"sigmaos/sigmaclnt/procclnt"
	sp "sigmaos/sigmap"
	"sigmaos/sigmasrv/memfssrv/memfs/inode"
	"sigmaos/sigmasrv/memfssrv/sigmapsrv/overlaydir"
	"sigmaos/sigmasrv/stats"
	spprotosrv "sigmaos/spproto/srv"
	"sigmaos/util/perf"
)

type SigmaPSrv struct {
	*spprotosrv.ProtSrvState
	*sesssrv.SessSrv
	pe          *proc.ProcEnv
	srvep       *sp.Tendpoint
	dirunder    fs.Dir
	dirover     *overlaydir.DirOverlay
	fencefs     fs.Dir
	stats       *stats.StatInode
	attachAuthF spprotosrv.AttachAuthF
}

func NewSigmaPSrv(pe *proc.ProcEnv, npc *dialproxyclnt.DialProxyClnt, root fs.Dir, addr *sp.Taddr, fencefs fs.Dir, aaf spprotosrv.AttachAuthF, exp sesssrv.ExpireI) *SigmaPSrv {
	psrv := &SigmaPSrv{
		pe:          pe,
		dirunder:    root,
		dirover:     overlaydir.MkDirOverlay(root),
		fencefs:     fencefs,
		attachAuthF: aaf,
	}
	psrv.stats = stats.NewStatsDev(inode.NewInodeAlloc(sp.DEV_STATFS))
	psrv.ProtSrvState = spprotosrv.NewProtSrvState(psrv.stats)
	psrv.VersionTable().Insert(fs.Uid(psrv.dirover))
	psrv.dirover.Mount(sp.STATSD, psrv.stats)
	psrv.SessSrv = sesssrv.NewSessSrv(pe, npc, addr, psrv.stats, psrv, exp)
	return psrv
}

func NewSigmaPSrvPost(root fs.Dir, pn string, addr *sp.Taddr, sc *sigmaclnt.SigmaClnt, fencefs fs.Dir, aaf spprotosrv.AttachAuthF, exp sesssrv.ExpireI) (*SigmaPSrv, string, error) {
	start := time.Now()
	psrv := NewSigmaPSrv(sc.ProcEnv(), sc.GetDialProxyClnt(), root, addr, fencefs, aaf, exp)
	perf.LogSpawnLatency("NewSigmaPSrv", sc.ProcEnv().GetPID(), sc.ProcEnv().GetSpawnTime(), start)
	start = time.Now()
	defer func() {
		perf.LogSpawnLatency("NewSigmaPSrvPost", sc.ProcEnv().GetPID(), sc.ProcEnv().GetSpawnTime(), start)
	}()
	if len(pn) > 0 {
		if mpn, err := psrv.postMount(sc, pn); err != nil {
			return nil, "", err
		} else {
			pn = mpn
		}
	}
	return psrv, pn, nil
}

func (psrv *SigmaPSrv) NewSession(p *sp.Tprincipal, sessid sessp.Tsession) sps.ProtSrv {
	return spprotosrv.NewProtServer(psrv.pe, psrv.ProtSrvState, p, sessid, psrv.GetRootCtx, psrv.attachAuthF)
}

func (psrv *SigmaPSrv) Root(p path.Tpathname) (fs.Dir, string, path.Tpathname) {
	d := psrv.dirunder
	if len(p) > 0 {
		o, err := psrv.dirover.Lookup(ctx.NewCtxNull(), p[0])
		if err == nil {
			return o.(fs.Dir), p[0], p[1:]
		}
	}
	return d, "", p
}

func (psrv *SigmaPSrv) Mount(name string, i fs.FsObj) {
	psrv.dirover.Mount(name, i)
}

func (psrv *SigmaPSrv) GetRootCtx(p *sp.Tprincipal, secrets map[string]*sp.SecretProto, aname string, sessid sessp.Tsession, clntid sp.TclntId) (fs.Dir, fs.CtxI) {
	return psrv.dirover, ctx.NewCtx(p, secrets, sessid, clntid, psrv.CondTable(), psrv.fencefs)
}

func (psrv *SigmaPSrv) GetSigmaPSrvEndpoint() *sp.Tendpoint {
	return psrv.srvep
}

func (psrv *SigmaPSrv) postMount(sc *sigmaclnt.SigmaClnt, pn string) (string, error) {
	start := time.Now()
	ep := psrv.GetEndpoint()
	psrv.srvep = ep
	db.DPrintf(db.BOOT, "Advertise %s at %v\n", pn, ep)
	if path.EndSlash(pn) {
		dir, err := sc.IsDir(pn)
		if err != nil {
			return "", err
		}
		if !dir {
			return "", serr.NewErr(serr.TErrNotDir, pn)
		}
		pn = mountPathName(pn, ep)
	}

	li, err := sc.LeaseClnt.AskLease(pn, fsetcd.LeaseTTL)
	if err != nil {
		return "", err
	}
	db.DPrintf(db.BOOT, "postMount AskLease[%v]: %v", time.Since(start), li.Lease())
	perf.LogSpawnLatency("SigmaPSrv.postMount.AskLease", sc.ProcEnv().GetPID(), sc.ProcEnv().GetSpawnTime(), start)
	start = time.Now()
	li.KeepExtending()
	perf.LogSpawnLatency("SigmaPSrv.postMount.KeepExtending", sc.ProcEnv().GetPID(), sc.ProcEnv().GetSpawnTime(), start)
	start = time.Now()

	if sc.ProcEnv().GetPrivileged() && sc.ProcEnv().GetHow() != proc.HMSCHED {
		// Make kproc semaphores here, so that they are leased (and don't cause hangs
		// on shutdown for kernel procs)
		if err := procclnt.MakeKProcSemaphores(sc.FsLib, sc.LeaseClnt); err != nil {
			db.DPrintf(db.ERROR, "Err make kproc semaphores: %v", err)
			return "", err
		}
	}
	start = time.Now()
	defer func() {
		perf.LogSpawnLatency("SigmaPSrv.postMount.MkLeasedEndpoint", sc.ProcEnv().GetPID(), sc.ProcEnv().GetSpawnTime(), start)
	}()

	if err := sc.MkLeasedEndpoint(pn, ep, li.Lease()); err != nil {
		return "", err
	}
	return pn, nil
}

// Return the pathname for posting in a directory of a service
func mountPathName(pn string, ep *sp.Tendpoint) string {
	return pn + "/" + ep.Addrs()[0].IPPort()
}
