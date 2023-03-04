package benchmarks_test

import (
	"time"

	"github.com/stretchr/testify/assert"

	db "sigmaos/debug"
	"sigmaos/kv"
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
	ckncore   proc.Tcore      // Number of exclusive cores allocated to each clerk.
	redis     bool            // True if this is a redis kv job.
	redisaddr string          // Redis server address.
	nkeys     int
	ready     chan bool
	*test.RealmTstate
}

func MakeKVJobInstance(ts *test.RealmTstate, nkvd int, kvdrepl int, nclerks []int, phases []time.Duration, ckdur string, kvdncore, ckncore proc.Tcore, auto string, redisaddr string) *KVJobInstance {
	ji := &KVJobInstance{RealmTstate: ts, job: rand.String(16)}

	kvf, err := kv.MakeKvdFleet(ts.SigmaClnt, ji.job, nkvd, kvdrepl, kvdncore, "0", auto)
	assert.Nil(ts.T, err)
	ji.kvf = kvf

	cm, err := kv.MkClerkMgr(ts.SigmaClnt, ji.job)
	assert.Nil(ts.T, err)
	ji.cm = cm

	ji.nclerks = nclerks
	ji.phases = phases
	ji.ckdur = ckdur
	ji.ckncore = ckncore
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

	assert.False(ts.T, ji.redis && ji.kvf.Nkvd() > 0, "Tried to run a kv job with both redis and sigma")
	return ji
}

func (ji *KVJobInstance) StartKVJob() {
	// Redis jobs don't require taht we start a balancer or any kvd groups.
	if ji.redis {
		return
	}
	db.DPrintf(db.TEST, "StartKVJob()")
	err := ji.kvf.Start()
	assert.Nil(ji.T, err)
	err = ji.cm.StartCmClerk()
	assert.Nil(ji.T, err)

	ji.cm.InitKeys(ji.nkeys)
}

// Returns true if there are no more phases left to execute.
func (ji *KVJobInstance) IsDone() bool {
	return ji.phase >= len(ji.nclerks)
}

// Perform the next phase of the job.
func (ji *KVJobInstance) NextPhase() {
	assert.False(ji.T, ji.IsDone(), "Tried to advance to another phase when already done.")
	// // Find out how far off we are from the desired number of clerks in this
	// // phase.
	// diff := len(ji.cpids) - ji.nclerks[ji.phase]
	// db.DPrintf(db.TEST, "Phase %v: diff %v", ji.phase, diff)
	// // While we have too many...
	// for ; diff > 0; diff-- {
	// 	ji.StopClerk()
	// }

	ji.cm.StartClerks(ji.ckdur, ji.nclerks[ji.phase])

	// While we have too few...
	//	for ; diff < 0; diff++ {
	//		ji.StartClerk()
	//	}
	// All clerks have started and can start doing ops.
	//ji.AllClerksStarted()
	// Make sure we got the number of clerks right.
	assert.Equal(ji.T, ji.cm.Nclerk(), ji.nclerks[ji.phase], "Didn't get right num of clerks for this phase: %v != %v", ji.cm.Nclerk(), ji.nclerks[ji.phase])
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
	// Remove all clerks.
	ji.cm.StopClerks()
	ji.kvf.Stop()
}

func (ji *KVJobInstance) GetKeyCountsPerGroup() map[string]int {
	return ji.cm.GetKeyCountsPerGroup(ji.nkeys)
}
