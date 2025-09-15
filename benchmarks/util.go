package benchmarks

import (
	"time"

	"github.com/stretchr/testify/assert"

	db "sigmaos/debug"
	"sigmaos/proc"
	mschedclnt "sigmaos/sched/msched/clnt"
	sp "sigmaos/sigmap"
	"sigmaos/test"
)

// Warm up a realm, by starting uprocds for it on all machines in the cluster.
func WarmupRealm(ts *test.RealmTstate, progs []string) (time.Time, int) {
	sdc := mschedclnt.NewMSchedClnt(ts.SigmaClnt.FsLib, sp.NOT_SET)
	// Get the list of mscheds.
	sds, err := sdc.GetMScheds()
	assert.Nil(ts.Ts.T, err, "Get MScheds: %v", err)
	db.DPrintf(db.TEST, "Warm up realm %v for progs %v mscheds %d %v", ts.GetRealm(), progs, len(sds), sds)
	start := time.Now()
	nDL := 0
	for _, kid := range sds {
		// Warm the cache for a binary
		for _, ptype := range []proc.Ttype{proc.T_LC, proc.T_BE} {
			for _, prog := range progs {
				err := sdc.WarmProcd(kid, ts.Ts.ProcEnv().GetPID(), ts.GetRealm(), prog+"-v"+sp.Version, ts.Ts.ProcEnv().GetSigmaPath(), ptype)
				nDL++
				assert.Nil(ts.Ts.T, err, "WarmProcd: %v", err)
			}
		}
	}
	db.DPrintf(db.TEST, "Warmed up realm %v", ts.GetRealm())
	return start, nDL
}
