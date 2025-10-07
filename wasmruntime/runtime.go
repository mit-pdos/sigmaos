package wasmruntime

import (
	"context"
	"sync"
	"sync/atomic"

	wasmer "github.com/wasmerio/wasmer-go/wasmer"

	db "sigmaos/debug"
	"sigmaos/proc"
	spproxyclnt "sigmaos/proxy/sigmap/clnt"
	sp "sigmaos/sigmap"
)

type Runtime struct {
	mu           sync.RWMutex
	instances    map[sp.Tpid]*Instance
	nextMockPid  int32
	precompStore *wasmer.Store
}

type WasmThread struct {
	pid    int
	done   chan error
	cancel context.CancelFunc
}

func NewRuntime() *Runtime {
	db.DPrintf(db.WASMRT, "Initializing WASM runtime")
	cfg := wasmer.NewConfig().UseCraneliftCompiler()
	engine := wasmer.NewEngineWithConfig(cfg)

	return &Runtime{
		instances: make(map[sp.Tpid]*Instance),
		// CR nmassri: figure out how to do actual pid management
		nextMockPid:  100000,
		precompStore: wasmer.NewStore(engine),
	}
}

func (rt *Runtime) SpawnInstance(uproc *proc.Proc, wasmPath string) (*WasmThread, error) {
	pid := uproc.GetPid()
	db.DPrintf(db.WASMRT, "[%v] SpawnInstance path=%s", pid, wasmPath)

	ctx, cancel := context.WithCancel(context.Background())

	db.DPrintf(db.WASMRT, "[%v] Creating sigmap client", pid)
	spClnt, err := spproxyclnt.NewSPProxyClnt(uproc.GetProcEnv(), nil)
	if err != nil {
		cancel()
		db.DPrintf(db.WASMRT_ERR, "[%v] Failed to create sigmap client: %v", pid, err)
		return nil, err
	}
	db.DPrintf(db.WASMRT, "[%v] Sigmap client created", pid)

	inst := &Instance{
		pid:      rt.allocMockPid(),
		uproc:    uproc,
		wasmPath: wasmPath,
		ctx:      ctx,
		cancel:   cancel,
		done:     make(chan error, 1),
		spClnt:   spClnt,
		rt:       rt,
	}

	rt.mu.Lock()
	rt.instances[pid] = inst
	rt.mu.Unlock()

	db.DPrintf(db.WASMRT, "[%v] Launching goroutine mockPID=%d", pid, inst.pid)
	go inst.run()

	return &WasmThread{
		pid:    inst.pid,
		done:   inst.done,
		cancel: cancel,
	}, nil
}

func (rt *Runtime) GetInstance(pid sp.Tpid) (*Instance, bool) {
	rt.mu.RLock()
	defer rt.mu.RUnlock()
	inst, ok := rt.instances[pid]
	return inst, ok
}

func (rt *Runtime) RemoveInstance(pid sp.Tpid) {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	delete(rt.instances, pid)
	db.DPrintf(db.WASMRT, "[%v] Instance removed", pid)
}

func (rt *Runtime) allocMockPid() int {
	return int(atomic.AddInt32(&rt.nextMockPid, 1))
}

func (wt *WasmThread) Wait() error {
	return <-wt.done
}

func (wt *WasmThread) Pid() int {
	return wt.pid
}

func (wt *WasmThread) Cancel() {
	wt.cancel()
}
