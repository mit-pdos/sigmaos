package benchmarks_test

import (
	"strconv"
	"time"

	"github.com/stretchr/testify/assert"

	db "ulambda/debug"
	"ulambda/group"
	"ulambda/groupmgr"
	"ulambda/kv"
	"ulambda/proc"
	"ulambda/test"
)

type KVJobInstance struct {
	nkvd    int             // Number of kvd groups to run the test with.
	phase   int             // Current phase of execution
	nclerks []int           // Number of clerks in each phase of the test.
	phases  []time.Duration // Duration of each phase of the test.
	balgm   *groupmgr.GroupMgr
	kvdgms  []*groupmgr.GroupMgr
	cpids   []proc.Tpid
	*test.Tstate
}

func MakeKVJobInstance(ts *test.Tstate, nkvd int, nclerks []int, phases []time.Duration) *KVJobInstance {
	ji := &KVJobInstance{}
	ji.nkvd = nkvd
	ji.phases = phases
	ji.nclerks = nclerks
	ji.kvdgms = []*groupmgr.GroupMgr{}
	ji.cpids = []proc.Tpid{}
	ji.Tstate = ts
	return ji
}

func (ji *KVJobInstance) StartKVJob() {
	// XXX auto or manual?
	ji.balgm = kv.StartBalancers(ji.FsLib, ji.ProcClnt, kv.NBALANCER, 0, "0", "manual")
	// Add an initial kvd group to put keys in.
	ji.AddKVDGroup()
	// Create keys
	_, err := kv.InitKeys(ji.FsLib, ji.ProcClnt)
	assert.Nil(ji.T, err, "InitKeys: %v", err)
}

// Returns true if there are no more phases left to execute.
func (ji *KVJobInstance) IsDone() bool {
	return ji.phase >= len(ji.phases)
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
	// Make sure we got the number of clerks right.
	assert.Equal(ji.T, len(ji.cpids), ji.nclerks[ji.phase], "Didn't get righ num of clerks for this phase: %v != %v", len(ji.cpids), ji.nclerks[ji.phase])
	// Sleep for the duration of this phase.
	db.DPrintf("TEST", "Phase %v: sleep", ji.phase)
	time.Sleep(ji.phases[ji.phase])
	db.DPrintf("TEST", "Phase %v: done", ji.phase)
	// Move to the  next phase
	ji.phase++
}

func (ji *KVJobInstance) AddKVDGroup() {
	// Name group
	grp := group.GRP + strconv.Itoa(len(ji.kvdgms))
	// Spawn group
	ji.kvdgms = append(ji.kvdgms, kv.SpawnGrp(ji.FsLib, ji.ProcClnt, grp, kv.KVD_REPL_LEVEL, 0))
	// Get balancer to add the group
	err := kv.BalancerOpRetry(ji.FsLib, "add", grp)
	assert.Nil(ji.T, err, "BalancerOp add: %v", err)
}

func (ji *KVJobInstance) RemoveKVDGroup() {
	n := len(ji.kvdgms) - 1
	// Get group nambe
	grp := group.GRP + strconv.Itoa(n)
	// Get balancer to remove the group
	err := kv.BalancerOpRetry(ji.FsLib, "del", grp)
	assert.Nil(ji.T, err, "BalancerOp del: %v", err)
	// Stop kvd group
	err = ji.kvdgms[n].Stop()
	assert.Nil(ji.T, err, "Stop kvd group: %v", err)
	// Remove kvd group
	ji.kvdgms = ji.kvdgms[:n]
}

func (ji *KVJobInstance) StartClerk() {
	pid, err := kv.StartClerk(ji.ProcClnt)
	assert.Nil(ji.T, err, "StartClerk: %v", err)
	ji.cpids = append(ji.cpids, pid)
}

func (ji *KVJobInstance) StopClerk() {
	var cpid proc.Tpid
	// Pop the first clerk pid.
	cpid, ji.cpids = ji.cpids[0], ji.cpids[1:]
	status, err := kv.StopClerk(ji.ProcClnt, cpid)
	assert.Nil(ji.T, err, "StopClerk: %v", err)
	assert.True(ji.T, status.IsStatusOK(), "Exit status: %v", status)
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
}
