package benchmarks

import (
	"time"

	"github.com/stretchr/testify/assert"

	db "sigmaos/debug"
	"sigmaos/proc"
	"sigmaos/proxy/wasm/rpc/wasmer"
	mschedclnt "sigmaos/sched/msched/clnt"
	sp "sigmaos/sigmap"
	"sigmaos/test"
)

// MRCoSandboxes is the co-sandbox scripts an MR job configured by cfg will run,
// for WarmupRealm to fetch onto every machine ahead of the job. Empty for a job
// that runs none.
func MRCoSandboxes(cfg *MRBenchConfig) []string {
	cs := make([]string, 0, 2)
	if cfg.UseCosandboxes {
		cs = append(cs, "mr_mapper_boot")
	}
	if cfg.UseCosandboxesReduce {
		cs = append(cs, "mr_reducer_boot")
	}
	return cs
}

// Warm up a realm, by starting uprocds for it on all machines in the cluster.
//
// coSandboxes, if any, are co-sandbox script names (e.g. "mr_mapper_boot") to
// fetch onto every machine alongside the binaries, so that the first proc using
// one doesn't pay for the download. Each is warmed once per (machine, proc
// type), matching where procd caches them.
func WarmupRealm(ts *test.RealmTstate, progs []string, coSandboxes ...string) (time.Time, int) {
	sdc := mschedclnt.NewMSchedClnt(ts.SigmaClnt.FsLib, sp.NOT_SET)
	// Get the list of mscheds.
	sds, err := sdc.GetMScheds()
	assert.Nil(ts.Ts.T, err, "Get MScheds: %v", err)
	db.DPrintf(db.TEST, "Warm up realm %v for progs %v cosandboxes %v mscheds %d %v", ts.GetRealm(), progs, coSandboxes, len(sds), sds)
	buildTag := ts.Ts.ProcEnv().BuildTag
	start := time.Now()
	nDL := 0
	for _, kid := range sds {
		// Warm the cache for a binary
		for _, ptype := range []proc.Ttype{proc.T_LC, proc.T_BE} {
			for _, prog := range progs {
				err := sdc.WarmProcd(kid, ts.Ts.ProcEnv().GetPID(), ts.GetRealm(), prog+"-v"+sp.Version, ts.Ts.ProcEnv().GetSigmaPath(), ptype, "")
				nDL++
				assert.Nil(ts.Ts.T, err, "WarmProcd: %v", err)
			}
			// Warm the cache for a co-sandbox. Sent as its own request rather
			// than piggybacked on a program's: the two sets are independent, and
			// pairing them would warm a co-sandbox once per program.
			for _, cs := range coSandboxes {
				err := sdc.WarmProcd(kid, ts.Ts.ProcEnv().GetPID(), ts.GetRealm(), progs[0]+"-v"+sp.Version, ts.Ts.ProcEnv().GetSigmaPath(), ptype, wasmer.CoSandboxPath(buildTag, cs))
				nDL++
				assert.Nil(ts.Ts.T, err, "WarmProcd cosandbox %v: %v", cs, err)
			}
		}
	}
	db.DPrintf(db.TEST, "Warmed up realm %v", ts.GetRealm())
	return start, nDL
}
