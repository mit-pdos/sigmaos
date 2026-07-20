package benchmarks_test

import (
	"sigmaos/apps/mr"
	db "sigmaos/debug"
	"sigmaos/proc"
	"sigmaos/test"
	"sigmaos/util/perf"
)

// MRStragglerJobInstance is an MRJobInstance that injects a single
// artificially-slow map task (slowTaskId, delayed by slowdownMs), to measure
// how much a straggler (as opposed to a failed task) hurts MR job completion
// time when nothing in SigmaOS today detects or mitigates it.
// PrepareMRJob/Wait/WaitJobExit are all reused unmodified via embedding;
// only StartMRJob is overridden.
type MRStragglerJobInstance struct {
	*MRJobInstance
	slowTaskId  int64
	slowdownMs  int
	specEnabled bool
}

func NewMRStragglerJobInstance(ts *test.RealmTstate, p *perf.Perf, app, jobRoot, jobname string, memreq proc.Tmem, slowTaskId int64, slowdownMs int, specEnabled bool) *MRStragglerJobInstance {
	return &MRStragglerJobInstance{
		MRJobInstance: NewMRJobInstance(ts, p, app, jobRoot, jobname, memreq),
		slowTaskId:    slowTaskId,
		slowdownMs:    slowdownMs,
		specEnabled:   specEnabled,
	}
}

func (ji *MRStragglerJobInstance) StartMRJob() {
	db.DPrintf(db.TEST, "Start MR job %v %v (straggler task %d +%dms, spec %v)", ji.jobname, ji.job, ji.slowTaskId, ji.slowdownMs, ji.specEnabled)
	ji.cm = mr.StartMRJob(ji.SigmaClnt, ji.jobRoot, ji.jobname, ji.job, ji.nmap, ji.memreq, 0, ji.mftid, ji.rftid, ji.slowTaskId, ji.slowdownMs, ji.specEnabled)
}
