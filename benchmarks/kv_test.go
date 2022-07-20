package benchmarks_test

import (
	"strconv"

	"github.com/stretchr/testify/assert"

	"ulambda/group"
	"ulambda/groupmgr"
	"ulambda/kv"
	"ulambda/proc"
	"ulambda/test"
)

type KVJobInstance struct {
	balgm  *groupmgr.GroupMgr
	kvdgms []*groupmgr.GroupMgr
	cpids  []proc.Tpid
	*test.Tstate
}

func MakeKVJobInstance(ts *test.Tstate) *KVJobInstance {
	ji := &KVJobInstance{}
	ji.kvdgms = []*groupmgr.GroupMgr{}
	ji.cpids = []proc.Tpid{}
	ji.Tstate = ts
	return ji
}

func MakeKVJob(ts *test.Tstate, auto string) *KVJobInstance {
	ji := MakeKVJobInstance(ts)
	ji.balgm = kv.StartBalancers(ji.FsLib, ji.ProcClnt, kv.NBALANCER, 0, "0", auto)
	// Add an initial kvd group to put keys in.
	ji.AddKVDGroup()
	// Create keys
	_, err := kv.InitKeys(ji.FsLib, ts.ProcClnt)
	assert.Nil(ts.T, err, "InitKeys: %v", err)
	return ji
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
