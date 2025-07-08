package spstats

type PathClntStats struct {
	Nsym       Tcounter
	Nopen      Tcounter
	NwalkPath  Tcounter
	NwalkElem  Tcounter
	NwalkOne   Tcounter
	NreadSym   Tcounter
	NwalkEP    Tcounter
	NwalkSym   Tcounter
	NwalkUnion Tcounter
	NmntNamed  Tcounter
	Nsession   Tcounter
}

// Make a SpStatsSnapshot from st while concurrent Inc()s may happen
func (pcst *PathClntStats) StatsSnapshot() *PathClntStatsSnapshot {
	stro := &PathClntStatsSnapshot{Counters: make(map[string]int64)}
	FillCounters(pcst, stro.Counters)
	return stro
}

type PathClntStatsSnapshot struct {
	Counters map[string]int64
}

func (pcst *PathClntStatsSnapshot) String() string {
	return StringCounters(pcst.Counters)
}
