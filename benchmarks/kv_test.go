package benchmarks_test

import (
	"path"
	"strconv"
	"time"

	"github.com/stretchr/testify/assert"

	db "ulambda/debug"
	"ulambda/group"
	"ulambda/groupmgr"
	"ulambda/kv"
	"ulambda/proc"
	"ulambda/rand"
	"ulambda/semclnt"
	"ulambda/test"
)

const (
	KEYS_PER_CLERK = 100
)

type KVJobInstance struct {
	nkvd     int             // Number of kvd groups to run the test with.
	kvdrepl  int             // kvd replication level
	phase    int             // Current phase of execution
	nclerks  []int           // Number of clerks in each phase of the test.
	phases   []time.Duration // Duration of each phase of the test.
	ck       *kv.KvClerk     // A clerk which can be used for initialization.
	ckdur    string          // Duration for which the clerk will do puts & gets.
	kvdncore proc.Tcore      // Number of exclusive cores allocated to each kvd.
	ckncore  proc.Tcore      // Number of exclusive cores allocated to each clerk.
	auto     string
	nkeys    int
	job      string
	ready    chan bool
	sem      *semclnt.SemClnt
	sempath  string
	balgm    *groupmgr.GroupMgr
	kvdgms   []*groupmgr.GroupMgr
	cpids    []proc.Tpid
	*test.Tstate
}

func MakeKVJobInstance(ts *test.Tstate, nkvd int, kvdrepl int, nclerks []int, phases []time.Duration, ckdur string, kvdncore, ckncore proc.Tcore, auto string) *KVJobInstance {
	ji := &KVJobInstance{}
	ji.nkvd = nkvd
	ji.kvdrepl = kvdrepl
	ji.nclerks = nclerks
	ji.phases = phases
	ji.ckdur = ckdur
	ji.kvdncore = kvdncore
	ji.ckncore = ckncore
	ji.job = rand.String(16)
	ji.auto = auto
	ji.ready = make(chan bool)
	ji.Tstate = ts
	// May already exit
	ji.MkDir(kv.KVDIR, 0777)
	// Should not exist.
	err := ji.MkDir(kv.JobDir(ji.job), 0777)
	assert.Nil(ts.T, err, "Make job dir: %v", err)
	if len(phases) == 0 {
		ji.sempath = path.Join(kv.JobDir(ji.job), "kvclerk-sem")
		ji.sem = semclnt.MakeSemClnt(ts.FsLib, ji.sempath)
		err = ji.sem.Init(0)
		assert.Nil(ts.T, err, "Sem init: %v", err)
	}
	ji.kvdgms = []*groupmgr.GroupMgr{}
	ji.cpids = []proc.Tpid{}
	// Find the maximum number of clerks ever set.
	maxNclerks := 0
	for _, nck := range nclerks {
		if maxNclerks < nck {
			maxNclerks = nck
		}
	}
	ji.nkeys = maxNclerks * KEYS_PER_CLERK
	return ji
}

func (ji *KVJobInstance) StartKVJob() {
	ji.balgm = kv.StartBalancers(ji.FsLib, ji.ProcClnt, ji.job, kv.NBALANCER, 0, ji.kvdncore, "0", ji.auto)
	// Add an initial kvd group to put keys in.
	ji.AddKVDGroup()
	// Create keys
	ck, err := kv.InitKeys(ji.FsLib, ji.ProcClnt, ji.job, ji.nkeys)
	assert.Nil(ji.T, err, "InitKeys: %v", err)
	ji.ck = ck
}

// Returns true if there are no more phases left to execute.
func (ji *KVJobInstance) IsDone() bool {
	return ji.phase >= len(ji.nclerks)
}

// Perform the next phase of the job.
func (ji *KVJobInstance) NextPhase() {
	assert.False(ji.T, ji.IsDone(), "Tried to advance to another phase when already done.")
	// Find out how far off we are from the desired number of clerks in this
	// phase.
	diff := len(ji.cpids) - ji.nclerks[ji.phase]
	db.DPrintf("TEST", "Phase %v: diff %v", ji.phase, diff)
	// While we have too many...
	for ; diff > 0; diff-- {
		ji.StopClerk()
	}
	// While we have too few...
	for ; diff < 0; diff++ {
		ji.StartClerk()
	}
	// All clerks have started and can start doing ops.
	ji.AllClerksStarted()
	// Make sure we got the number of clerks right.
	assert.Equal(ji.T, len(ji.cpids), ji.nclerks[ji.phase], "Didn't get righ num of clerks for this phase: %v != %v", len(ji.cpids), ji.nclerks[ji.phase])
	// Sleep for the duration of this phase.
	db.DPrintf("TEST", "Phase %v: sleep", ji.phase)
	// If running with unbounded clerks, sleep for a bit.
	if len(ji.phases) > 0 {
		time.Sleep(ji.phases[ji.phase])
	} else {
		// Otherwise, wait for bounded clerks to finish.
		ji.WaitForClerks()
	}
	db.DPrintf("TEST", "Phase %v: done", ji.phase)
	// Move to the  next phase
	ji.phase++
}

func (ji *KVJobInstance) AllClerksStarted() {
	if len(ji.phases) > 0 {
		return
	}
	ji.sem.Up()
}

// If running with bounded clerks, wait for clerks to run.
func (ji *KVJobInstance) WaitForClerks() {
	aggTpt := float64(0)
	for _, cpid := range ji.cpids {
		status, err := ji.WaitExit(cpid)
		assert.Nil(ji.T, err, "StopClerk: %v", err)
		assert.True(ji.T, status.IsStatusOK(), "Exit status: %v", status)
		tpt := status.Data().(float64)
		aggTpt += tpt
		db.DPrintf(db.ALWAYS, "Ops/sec: %v", tpt)
	}
	db.DPrintf(db.ALWAYS, "Aggregate throughput: %v (ops/sec)", aggTpt)
	ji.cpids = ji.cpids[:0]
}

func (ji *KVJobInstance) AddKVDGroup() {
	// Name group
	grp := group.GRP + strconv.Itoa(len(ji.kvdgms))
	// Spawn group
	ji.kvdgms = append(ji.kvdgms, kv.SpawnGrp(ji.FsLib, ji.ProcClnt, ji.job, grp, ji.kvdncore, ji.kvdrepl, 0))
	// Get balancer to add the group
	err := kv.BalancerOpRetry(ji.FsLib, ji.job, "add", grp)
	assert.Nil(ji.T, err, "BalancerOp add: %v", err)
}

func (ji *KVJobInstance) RemoveKVDGroup() {
	n := len(ji.kvdgms) - 1
	// Get group nambe
	grp := group.GRP + strconv.Itoa(n)
	// Get balancer to remove the group
	err := kv.BalancerOpRetry(ji.FsLib, ji.job, "del", grp)
	assert.Nil(ji.T, err, "BalancerOp del: %v", err)
	// Stop kvd group
	err = ji.kvdgms[n].Stop()
	assert.Nil(ji.T, err, "Stop kvd group: %v", err)
	// Remove kvd group
	ji.kvdgms = ji.kvdgms[:n]
}

func (ji *KVJobInstance) StartClerk() {
	idx := len(ji.cpids)
	var args []string
	if len(ji.phases) > 0 {
		args = nil
	} else {
		// Each clerk puts/sets kv.NKEYS, so offset their puts/sets by idx * kv.NKEYS
		args = append(args, ji.ckdur, strconv.Itoa(idx*kv.NKEYS), ji.sempath)
	}
	db.DPrintf("TEST", "Spawn clerk")
	var pid proc.Tpid
	var err error
	for {
		pid, err = kv.StartClerk(ji.ProcClnt, ji.job, args, ji.ckncore)
		if err == nil {
			break
		}
	}
	assert.Nil(ji.T, err, "StartClerk: %v", err)
	db.DPrintf("TEST", "Done spawning clerk %v", pid)
	ji.cpids = append(ji.cpids, pid)
}

func (ji *KVJobInstance) StopClerk() {
	var cpid proc.Tpid
	// Pop the first clerk pid.
	cpid, ji.cpids = ji.cpids[0], ji.cpids[1:]
	status, err := kv.StopClerk(ji.ProcClnt, cpid)
	assert.Nil(ji.T, err, "StopClerk: %v", err)
	assert.True(ji.T, status.IsStatusEvicted(), "Exit status: %v", status)
}

func (ji *KVJobInstance) GetKeyCountsPerGroup() map[string]int {
	keys := make([]kv.Tkey, 0, ji.nkeys)
	for i := uint64(0); i < uint64(ji.nkeys); i++ {
		keys = append(keys, kv.MkKey(i))
	}
	return ji.ck.GetKeyCountsPerGroup(keys)
}

func (ji *KVJobInstance) Stop() {
	// Remove all clerks.
	nclerks := len(ji.cpids)
	for i := 0; i < nclerks; i++ {
		ji.StopClerk()
	}
	// Remove all but one kvd group.
	nkvds := len(ji.kvdgms)
	for i := 0; i < nkvds-1; i++ {
		ji.RemoveKVDGroup()
	}
	// Stop the balancers.
	ji.balgm.Stop()
	// Remove the last kvd group after removing the balancer.
	ji.kvdgms[0].Stop()
	ji.kvdgms = nil
	//	err := kv.RemoveJob(ji.FsLib, ji.job)
	//	assert.Nil(ji.T, err, "Remove job: %v", err)
}
