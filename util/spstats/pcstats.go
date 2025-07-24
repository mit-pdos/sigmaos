package spstats

type PathClntStats struct {
	Nsym         Tcounter
	Nopen        Tcounter
	NwalkPath    Tcounter
	NwalkElem    Tcounter
	NwalkOne     Tcounter
	NreadSym     Tcounter
	NwalkEP      Tcounter
	NwalkSym     Tcounter
	NwalkUnion   Tcounter
	NgetNamedOK  Tcounter
	NgetNamedErr Tcounter
	NmntNamedOK  Tcounter
	NmntNamedErr Tcounter
	Nsession     Tcounter
	NnetclntOK   Tcounter
	NnetclntErr  Tcounter
}

// Make a SpStatsSnapshot from st while concurrent Inc()s may happen
func (pcst *PathClntStats) StatsSnapshot() *TcounterSnapshot {
	stro := NewTcounterSnapshot()
	stro.FillCounters(pcst)
	return stro
}
