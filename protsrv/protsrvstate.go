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
	cct := clntcond.NewClntCondTable()
	pss := &ProtSrvState{
		stats: stats,
		et:    ephemeralmap.NewEphemeralMap(),
		plt:   lockmap.NewPathLockTable(),
		cct:   cct,
		wt:    watch.NewWatchTable(cct),
		vt:    version.NewVersionTable(),
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
