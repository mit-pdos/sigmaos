package benchmarks

import (
	"os"
	"strings"
	"testing"
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

func debugSelectorsSet(s string) map[string]bool {
	out := make(map[string]bool)
	for _, p := range strings.Split(s, ";") {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		out[p] = true
	}
	return out
}

func EnsureSigmaDebugEnabled(t *testing.T, selectors ...string) {
	t.Helper()
	prev, hadPrev := os.LookupEnv("SIGMADEBUG")
	sel := debugSelectorsSet(prev)
	for _, s := range selectors {
		sel[s] = true
	}

	keys := make([]string, 0, len(sel))
	for k := range sel {
		keys = append(keys, k)
	}
	updated := strings.Join(keys, ";")
	if updated != "" {
		updated += ";"
	}
	if err := os.Setenv("SIGMADEBUG", updated); err != nil {
		t.Fatalf("set SIGMADEBUG: %v", err)
	}
	t.Cleanup(func() {
		if hadPrev {
			_ = os.Setenv("SIGMADEBUG", prev)
		} else {
			_ = os.Unsetenv("SIGMADEBUG")
		}
	})
}
