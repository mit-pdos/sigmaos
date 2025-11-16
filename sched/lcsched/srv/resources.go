package srv

import (
	db "sigmaos/debug"
	"sigmaos/proc"
	sp "sigmaos/sigmap"
)

const (
	// MCPU in a queueable proc resource pool dedicated to running boot scripts
	POOL_BOOT_SCRIPT_MCPU proc.Tmcpu = 1000
	POOL_BOOT_SCRIPT_MEM  proc.Tmem  = 0
)

type Resources struct {
	maxmcpu   proc.Tmcpu
	maxmem    proc.Tmem
	mcpu      proc.Tmcpu
	mem       proc.Tmem
	qprpID    uint64
	qprps     map[*QueueableProcResourcePool]bool    // Queueable proc resource pools
	pidToPool map[sp.Tpid]*QueueableProcResourcePool // Mappings of procs to queueable resource pools
}

func newResources(mcpuInt uint32, memInt uint32) *Resources {
	return &Resources{
		maxmcpu:   proc.Tmcpu(mcpuInt),
		maxmem:    proc.Tmem(memInt),
		mcpu:      proc.Tmcpu(mcpuInt),
		mem:       proc.Tmem(memInt),
		qprpID:    0,
		qprps:     make(map[*QueueableProcResourcePool]bool),
		pidToPool: make(map[sp.Tpid]*QueueableProcResourcePool),
	}
}

// Caller holds lock
func (r *Resources) alloc(p *proc.Proc) {
	defer r.sanityCheck()

	// If this is a queueable proc, get a pool in which to run it
	if p.GetIsQueueable() {
		// Check if there is room in one of the existing queueable proc resource
		// pools
		pool, ok := r.getQueueableResourcePool(p)
		if !ok {
			// No room for the proc in existing pools, so create a new one
			// First, Calculate resources to dedicate to this pool
			newPoolMcpu := POOL_BOOT_SCRIPT_MCPU + p.GetMcpu()
			newPoolMem := POOL_BOOT_SCRIPT_MEM + p.GetMem()
			// Allocate the pool's resources
			r.mcpu -= newPoolMcpu
			r.mem -= newPoolMem
			// Create the pool
			r.qprpID++
			pool = newQueueableProcResourcePool(r.qprpID, POOL_BOOT_SCRIPT_MCPU, POOL_BOOT_SCRIPT_MEM, p.GetMcpu(), p.GetMem())
			db.DPrintf(db.LCSCHED, "New resource pool %v", pool.GetID())
		}
		db.DPrintf(db.LCSCHED, "[%v] Assign proc to resource pool %v", p.GetPid(), pool.GetID())
		// Assign this proc to the pool, and allocate resources in the pool
		r.pidToPool[p.GetPid()] = pool
		pool.Alloc(p)
	} else {
		r.mcpu -= p.GetMcpu()
		r.mem -= p.GetMem()
	}
}

// Caller holds lock
func (r *Resources) free(p *proc.Proc) {
	defer r.sanityCheck()

	if p.GetIsQueueable() {
		pool := r.pidToPool[p.GetPid()]
		db.DPrintf(db.LCSCHED, "[%v] Remove proc from resource pool %v", p.GetPid(), pool.GetID())
		isEmpty := pool.Free(p)
		if isEmpty {
			// Pool is now empty, so delete it
			delete(r.qprps, pool)
			// Get the resources that were allocated to it, and free them
			mcpu, mem := pool.GetResources()
			r.mcpu += mcpu
			r.mem += mem
			db.DPrintf(db.LCSCHED, "Resource pool %v empty, deleting", pool.GetID())
		}
	} else {
		r.mcpu += p.GetMcpu()
		r.mem += p.GetMem()
	}
}

// Caller holds lock
func (r *Resources) sanityCheck() {
	if r.mcpu < 0 || r.mem < 0 {
		db.DFatalf("Invalid mcpu (%v) or mem (%v): too little", r.mcpu, r.mem)
	}
	if r.mcpu > r.maxmcpu || r.mem > r.maxmem {
		db.DFatalf("Invalid mcpu (%v) or mem (%v): too much", r.mcpu, r.mem)
	}
}

// Caller holds lock
func (r *Resources) isEligible(p *proc.Proc) bool {
	if p.GetIsQueueable() {
		// Check if there is room in one of the existing queueable proc resource
		// pools
		_, ok := r.getQueueableResourcePool(p)
		if ok {
			return true
		}
		// No pool with room found. Check if there are enough resources to create
		// a new pool to run the proc & its boot script.
		newPoolMcpu := POOL_BOOT_SCRIPT_MCPU + p.GetMcpu()
		newPoolMem := POOL_BOOT_SCRIPT_MEM + p.GetMem()
		if newPoolMem <= r.mem && newPoolMcpu <= r.mcpu {
			return true
		}
	} else {
		if p.GetMem() <= r.mem && p.GetMcpu() <= r.mcpu {
			return true
		}
	}
	return false
}

func (r *Resources) getQueueableResourcePool(p *proc.Proc) (*QueueableProcResourcePool, bool) {
	for pool, _ := range r.qprps {
		// If there is room for this proc in the pool, return the pool
		if pool.IsEligible(p) {
			return pool, true
		}
	}
	return nil, false
}

// Pool of resources allocated to run bootscripts & queueable procs
type QueueableProcResourcePool struct {
	id                uint64
	maxBootScriptMcpu proc.Tmcpu // Max (initial) CPU resources for running queueable procs in this pool
	maxBootScriptMem  proc.Tmem  // Max (initial) Mem resources for running queueable procs in this pool
	bootScriptMcpu    proc.Tmcpu // CPU resources for running bootscripts
	bootScriptMem     proc.Tmem  // Mem resources for running bootscripts
	procMcpu          proc.Tmcpu // CPU resources for running queueable procs which share the pool
	procMem           proc.Tmem  // Mem resources for running queueable procs which share the pool
	nproc             int        // Number of procs allocated to this pool
}

func newQueueableProcResourcePool(id uint64, bootScriptMcpu proc.Tmcpu, bootScriptMem proc.Tmem, procMcpu proc.Tmcpu, procMem proc.Tmem) *QueueableProcResourcePool {
	return &QueueableProcResourcePool{
		id:                id,
		maxBootScriptMcpu: bootScriptMcpu,
		maxBootScriptMem:  bootScriptMem,
		bootScriptMcpu:    bootScriptMcpu,
		bootScriptMem:     bootScriptMem,
		procMcpu:          procMcpu,
		procMem:           procMem,
		nproc:             0,
	}
}

func (qprp *QueueableProcResourcePool) Alloc(p *proc.Proc) {
	defer qprp.sanityCheck()

	// Update resources available to run bootscripts in this resource pool
	qprp.bootScriptMcpu -= p.GetBootScriptMcpu()
	qprp.bootScriptMem -= p.GetBootScriptMem()
	// Update number of running procs assigned to this resource pool
	qprp.nproc++
	p.SetQueueableResourcePoolID(qprp.id)
}

// Return true if pool is now empty
func (qprp *QueueableProcResourcePool) Free(p *proc.Proc) bool {
	defer qprp.sanityCheck()

	// Update resources available to run bootscripts in this resource pool
	qprp.bootScriptMcpu -= p.GetBootScriptMcpu()
	qprp.bootScriptMem -= p.GetBootScriptMem()
	// Update number of running procs assigned to this resource pool
	qprp.nproc--
	return qprp.nproc == 0
}

func (qprp *QueueableProcResourcePool) sanityCheck() {
	if qprp.bootScriptMcpu < 0 || qprp.bootScriptMem < 0 {
		db.DFatalf("Invalid bootscript mcpu (%v) or mem (%v): too little", qprp.bootScriptMcpu, qprp.bootScriptMem)
	}
	if qprp.bootScriptMcpu > qprp.maxBootScriptMcpu || qprp.bootScriptMem > qprp.maxBootScriptMem {
		db.DFatalf("Invalid bootscript mcpu (%v) or mem (%v): too much", qprp.bootScriptMcpu, qprp.bootScriptMem)
	}
	if qprp.nproc == 0 && (qprp.bootScriptMcpu != qprp.maxBootScriptMcpu || qprp.bootScriptMem != qprp.maxBootScriptMem) {
		db.DFatalf("No procs allocated to pool, but boot script mcpu (%v) or mem (%v) not replenished", qprp.bootScriptMcpu, qprp.bootScriptMem)
	}
}

func (qprp *QueueableProcResourcePool) IsEligible(p *proc.Proc) bool {
	// If there is room in the pool to run the boot script, and the proc component of the pool is large enough to run the queueable proc, then the proc is eligible to run on this pool
	if p.GetBootScriptMcpu() <= qprp.bootScriptMcpu && p.GetBootScriptMem() <= qprp.bootScriptMem &&
		p.GetMcpu() <= qprp.procMcpu && p.GetMem() <= qprp.procMem {
		return true
	}
	return false
}

func (qprp *QueueableProcResourcePool) GetResources() (proc.Tmcpu, proc.Tmem) {
	return qprp.procMcpu + qprp.maxBootScriptMcpu, qprp.procMem + qprp.maxBootScriptMem
}

func (qprp *QueueableProcResourcePool) GetID() uint64 {
	return qprp.id
}
