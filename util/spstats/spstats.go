package spstats

import (
	db "sigmaos/debug"
	sessp "sigmaos/session/proto"
)

type SpStats struct {
	Ntotal      Tcounter
	Nversion    Tcounter
	Nauth       Tcounter
	Nattach     Tcounter
	Ndetach     Tcounter
	Nflush      Tcounter
	Nwalk       Tcounter
	Nclunk      Tcounter
	Nopen       Tcounter
	Nwatch      Tcounter
	Ncreate     Tcounter
	Nread       Tcounter
	Nwrite      Tcounter
	Nremove     Tcounter
	Nremovefile Tcounter
	Nstat       Tcounter
	Nwstat      Tcounter
	Nrenameat   Tcounter
	Nget        Tcounter
	Nput        Tcounter
	Nrpc        Tcounter
}

func (st *SpStats) Inc(fct sessp.Tfcall, ql int64) {
	switch fct {
	case sessp.TTversion:
		Inc(&st.Nversion, 1)
	case sessp.TTauth:
		Inc(&st.Nauth, 1)
	case sessp.TTattach:
		Inc(&st.Nattach, 1)
	case sessp.TTdetach:
		Inc(&st.Ndetach, 1)
	case sessp.TTflush:
		Inc(&st.Nflush, 1)
	case sessp.TTwalk:
		Inc(&st.Nwalk, 1)
	case sessp.TTopen:
		Inc(&st.Nopen, 1)
	case sessp.TTcreate:
		Inc(&st.Ncreate, 1)
	case sessp.TTread, sessp.TTreadF:
		Inc(&st.Nread, 1)
	case sessp.TTwrite, sessp.TTwriteF:
		Inc(&st.Nwrite, 1)
	case sessp.TTclunk:
		Inc(&st.Nclunk, 1)
	case sessp.TTremove:
		Inc(&st.Nremove, 1)
	case sessp.TTremovefile:
		Inc(&st.Nremovefile, 1)
	case sessp.TTstat:
		Inc(&st.Nstat, 1)
	case sessp.TTwstat:
		Inc(&st.Nwstat, 1)
	case sessp.TTwatch:
		Inc(&st.Nwatch, 1)
	case sessp.TTrenameat:
		Inc(&st.Nrenameat, 1)
	case sessp.TTgetfile:
		Inc(&st.Nget, 1)
	case sessp.TTputfile:
		Inc(&st.Nput, 1)
	case sessp.TTwriteread:
		Inc(&st.Nrpc, 1)
	default:
		db.DPrintf(db.ALWAYS, "StatInfo: missing counter for %v\n", fct)
	}
	Inc(&st.Ntotal, 1)
}

// For reading and marshaling
type SpStatsSnapshot struct {
	Counters map[string]int64
}

// Make a SpStatsSnapshot from st while concurrent Inc()s may happen
func (st *SpStats) StatsSnapshot() *SpStatsSnapshot {
	stro := &SpStatsSnapshot{Counters: make(map[string]int64)}
	FillCounters(st, stro.Counters)
	return stro
}

func (st *SpStatsSnapshot) String() string {
	return StringCounters(st.Counters)
}
