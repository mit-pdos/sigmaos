package benchmarks_test

import (
	"time"

	"github.com/stretchr/testify/assert"

	"sigmaos/apps/kv"
	db "sigmaos/debug"
	"sigmaos/proc"
	"sigmaos/rand"
	"sigmaos/test"
)

const (
	KEYS_PER_CLERK = 100
)

type KVJobInstance struct {
	job       string
	kvf       *kv.KVFleet
	cm        *kv.ClerkMgr
	phase     int             // Current phase of execution
	nclerks   []int           // Number of clerks in each phase of the test.
	phases    []time.Duration // Duration of each phase of the test.
	ckdur     string          // Duration for which the clerk will do puts & gets.
	redis     bool            // True if this is a redis kv job.
	redisaddr string          // Redis server address.
	nkeys     int
	ready     chan bool
	*test.RealmTstate
}

func NewKVJobInstance(ts *test.RealmTstate, nkvd int, kvdrepl int, nclerks []int, phases []time.Duration, ckdur string, kvdmcpu, ckmcpu proc.Tmcpu, auto string, redisaddr string) *KVJobInstance {
	ji := &KVJobInstance{RealmTstate: ts, job: rand.String(16)}

	kvf, err := kv.NewKvdFleet(ts.SigmaClnt, ji.job, 0, nkvd, kvdrepl, 0, kvdmcpu, "0", auto)
	assert.Nil(ts.Ts.T, err)
	ji.kvf = kvf

	cm, err := kv.NewClerkMgr(ts.SigmaClnt, ji.job, ckmcpu, false)
	assert.Nil(ts.Ts.T, err)
	ji.cm = cm

	ji.nclerks = nclerks
	ji.phases = phases
	ji.ckdur = ckdur
	ji.redis = redisaddr != ""
	ji.redisaddr = redisaddr
	ji.ready = make(chan bool)
	ji.RealmTstate = ts

	// Find the maximum number of clerks ever set.
	maxNclerks := 0
	for _, nck := range nclerks {
		if maxNclerks < nck {
			maxNclerks = nck
		}
	}
	ji.nkeys = maxNclerks * KEYS_PER_CLERK

	assert.False(ts.Ts.T, ji.redis && ji.kvf.Nkvd() > 0, "Tried to run a kv job with both redis and sigma")
	return ji
}

func (ji *KVJobInstance) StartKVJob() {
	// Redis jobs don't require taht we start a balancer or any kvd groups.
	if ji.redis {
		return
	}
	db.DPrintf(db.TEST, "StartKVJob()")
	err := ji.kvf.Start()
	assert.Nil(ji.Ts.T, err)
	err = ji.cm.StartCmClerk()
	assert.Nil(ji.Ts.T, err)

	ji.cm.InitKeys(ji.nkeys)
}

// Returns true if there are no more phases left to execute.
func (ji *KVJobInstance) IsDone() bool {
	return ji.phase >= len(ji.nclerks)
}

// Perform the next phase of the job.
func (ji *KVJobInstance) NextPhase() {
	assert.False(ji.Ts.T, ji.IsDone(), "Tried to advance to another phase when already done.")
	// Find out how far off we are from the desired number of clerks in this
	// phase.

	diff := ji.nclerks[ji.phase] - ji.cm.Nclerk()

	db.DPrintf(db.TEST, "Phase %v: diff %v", ji.phase, diff)

	ji.cm.AddClerks(ji.ckdur, ji.nclerks[ji.phase])

	// Make sure we got the number of clerks right.

	assert.Equal(ji.Ts.T, ji.cm.Nclerk(), ji.nclerks[ji.phase], "Didn't get right num of clerks for this phase: %v != %v", ji.cm.Nclerk(), ji.nclerks[ji.phase])

	// Sleep for the duration of this phase.
	db.DPrintf(db.TEST, "Phase %v: sleep", ji.phase)

	// If running with unbounded clerks, sleep for a bit.
	if len(ji.phases) > 0 {
		time.Sleep(ji.phases[ji.phase])
		ji.cm.StopClerks()
	} else {
		// Otherwise, wait for bounded clerks to finish.
		ji.cm.WaitForClerks()
	}
	db.DPrintf(db.TEST, "Phase %v: done", ji.phase)
	// Move to the  next phase
	ji.phase++
}

func (ji *KVJobInstance) Stop() {
	ji.cm.StopClerks()
	ji.kvf.Stop()
}

func (ji *KVJobInstance) GetKeyCountsPerGroup() map[string]int {
	return ji.cm.GetKeyCountsPerGroup(ji.nkeys)
}
