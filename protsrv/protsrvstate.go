package protsrv

import (
	"sigmaos/clntcond"
	"sigmaos/ephemeralmap"
	"sigmaos/lockmap"
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
}

func NewProtSrvState(stats *stats.StatInfo) *ProtSrvState {
	pss := &ProtSrvState{stats: stats}
	pss.et = ephemeralmap.NewEphemeralMap()
	pss.plt = lockmap.NewPathLockTable()
	pss.cct = clntcond.NewClntCondTable()
	pss.wt = watch.NewWatchTable(pss.cct)
	pss.vt = version.NewVersionTable()
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
