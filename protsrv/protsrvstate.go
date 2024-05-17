package protsrv

import (
	"sigmaos/auth"
	"sigmaos/clntcond"
	db "sigmaos/debug"
	"sigmaos/ephemeralmap"
	"sigmaos/fs"
	"sigmaos/lockmap"
	"sigmaos/path"
	"sigmaos/serr"
	sp "sigmaos/sigmap"
	"sigmaos/stats"
	"sigmaos/version"
	"sigmaos/watch"
)

type ProtSrvState struct {
	plt   *lockmap.PathLockTable
	wt    *watch.WatchTable
	vt    *version.VersionTable
	stats *stats.StatInfo
	et    *ephemeralmap.EphemeralMap
	cct   *clntcond.ClntCondTable
	auth  auth.AuthMgr
}

func NewProtSrvState(amgr auth.AuthMgr, stats *stats.StatInfo) *ProtSrvState {
	cct := clntcond.NewClntCondTable()
	pss := &ProtSrvState{
		stats: stats,
		et:    ephemeralmap.NewEphemeralMap(),
		plt:   lockmap.NewPathLockTable(),
		cct:   cct,
		wt:    watch.NewWatchTable(cct),
		vt:    version.NewVersionTable(),
		auth:  amgr,
	}
	return pss
}

func (pss *ProtSrvState) CondTable() *clntcond.ClntCondTable {
	return pss.cct
}

func (pss *ProtSrvState) VersionTable() *version.VersionTable {
	return pss.vt
}

func (pss *ProtSrvState) PathLockTable() *lockmap.PathLockTable {
	return pss.plt
}

func (pss *ProtSrvState) EphemeralMap() *ephemeralmap.EphemeralMap {
	return pss.et
}

func (pss *ProtSrvState) Stats() *stats.StatInfo {
	return pss.stats
}

func (pss *ProtSrvState) RemoveObj(ctx fs.CtxI, o fs.FsObj, path path.Path, f sp.Tfence) *serr.Err {
	name := path.Base()
	if name == "." {
		return serr.NewErr(serr.TErrInval, name)
	}

	// lock path to make WatchV and Remove interact correctly
	dlk := pss.plt.Acquire(ctx, path.Dir(), lockmap.WLOCK)
	flk := pss.plt.Acquire(ctx, path, lockmap.WLOCK)
	defer pss.plt.ReleaseLocks(ctx, dlk, flk, lockmap.WLOCK)

	pss.stats.IncPathString(flk.Path())

	db.DPrintf(db.PROTSRV, "%v: removeObj %v %v", ctx.ClntId(), name, o)

	// Call before Remove(), because after remove o's underlying
	// object may not exist anymore.
	ephemeral := o.Perm().IsEphemeral()
	if err := o.Parent().Remove(ctx, name, f); err != nil {
		return err
	}

	pss.vt.IncVersion(o.Path())
	pss.vt.IncVersion(o.Parent().Path())

	pss.wt.WakeupWatch(flk)
	pss.wt.WakeupWatch(dlk)

	if ephemeral && pss.et != nil {
		pss.et.Delete(path.String())
	}
	return nil
}
