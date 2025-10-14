package wasmruntime

import (
	"context"
	"fmt"
	"os"
	"time"

	wasmer "github.com/wasmerio/wasmer-go/wasmer"

	db "sigmaos/debug"
	"sigmaos/proc"
	spproxyclnt "sigmaos/proxy/sigmap/clnt"
	"sigmaos/util/perf"
)

const SHARED_BUF_SZ = 655360

type Instance struct {
	pid             int
	uproc           *proc.Proc
	wasmPath        string
	ctx             context.Context
	cancel          context.CancelFunc
	done            chan error
	spClnt          *spproxyclnt.SPProxyClnt
	rt              *Runtime
	store           *wasmer.Store
	instance        *wasmer.Instance
	wasmBufPtr      int32
	instantiateTime time.Time
}

func (inst *Instance) run() {
	defer func() {
		inst.cleanup()
		inst.rt.RemoveInstance(inst.uproc.GetPid())
	}()

	pid := inst.uproc.GetPid()
	db.DPrintf(db.WASMRT, "[%v] Instance goroutine started", pid)

	err := inst.executeWasm()
	if err != nil {
		db.DPrintf(db.WASMRT_ERR, "[%v] Execution failed: %v", pid, err)
	}

	inst.done <- err
	close(inst.done)
}

func (inst *Instance) executeWasm() error {
	pid := inst.uproc.GetPid()
	t_overall := time.Now()

	t_read := time.Now()
	db.DPrintf(db.WASMRT, "[%v] Reading WASM file: %s", pid, inst.wasmPath)
	wasmBytes, err := os.ReadFile(inst.wasmPath)
	if err != nil {
		db.DPrintf(db.WASMRT_ERR, "[%v] File read failed: %v", pid, err)
		return fmt.Errorf("read file: %w", err)
	}
	perf.LogSpawnLatency("WASM file read", pid, perf.TIME_NOT_SET, t_read)
	db.DPrintf(db.ALWAYS, "[%v] WASM file read: %d bytes in %v", pid, len(wasmBytes), time.Since(t_read))

	t_engine := time.Now()
	db.DPrintf(db.WASMRT, "[%v] Creating Wasmer engine", pid)
	engine := wasmer.NewEngine()
	inst.store = wasmer.NewStore(engine)
	db.DPrintf(db.ALWAYS, "[%v] Engine creation: %v", pid, time.Since(t_engine))

	t_compile := time.Now()
	db.DPrintf(db.WASMRT, "[%v] Compiling module", pid)
	module, err := wasmer.NewModule(inst.store, wasmBytes)
	if err != nil {
		db.DPrintf(db.WASMRT_ERR, "[%v] Compilation failed: %v", pid, err)
		return fmt.Errorf("compile module: %w", err)
	}
	perf.LogSpawnLatency("WASM compilation", pid, inst.uproc.GetSpawnTime(), t_compile)
	db.DPrintf(db.ALWAYS, "[%v] WASM compilation: %v", pid, time.Since(t_compile))

	t_wasi := time.Now()
	db.DPrintf(db.WASMRT, "[%v] Creating WASI environment", pid)
	wasiEnv, err := wasmer.NewWasiStateBuilder("wasm-runtime").
		CaptureStdout().
		Finalize()
	if err != nil {
		db.DPrintf(db.WASMRT_ERR, "[%v] WASI env creation failed: %v", pid, err)
		return fmt.Errorf("create wasi env: %w", err)
	}
	db.DPrintf(db.ALWAYS, "[%v] WASI env creation: %v", pid, time.Since(t_wasi))

	t_imports := time.Now()
	db.DPrintf(db.WASMRT, "[%v] Generating WASI imports", pid)
	wasiImports, err := wasiEnv.GenerateImportObject(inst.store, module)
	if err != nil {
		db.DPrintf(db.WASMRT_ERR, "[%v] WASI import generation failed: %v", pid, err)
		return fmt.Errorf("generate wasi imports: %w", err)
	}
	db.DPrintf(db.ALWAYS, "[%v] WASI imports generation: %v", pid, time.Since(t_imports))

	t_hostfuncs := time.Now()
	db.DPrintf(db.WASMRT, "[%v] Creating host functions", pid)
	importObject := inst.createHostFunctions(wasiImports)
	db.DPrintf(db.ALWAYS, "[%v] Host functions creation: %v", pid, time.Since(t_hostfuncs))

	t_instantiate := time.Now()
	inst.instantiateTime = time.Now()
	db.DPrintf(db.WASMRT, "[%v] Instantiating module", pid)
	inst.instance, err = wasmer.NewInstance(module, importObject)
	if err != nil {
		db.DPrintf(db.WASMRT_ERR, "[%v] Instantiation failed: %v", pid, err)
		return fmt.Errorf("instantiate: %w", err)
	}
	perf.LogSpawnLatency("WASM instantiation", pid, inst.uproc.GetSpawnTime(), t_instantiate)
	db.DPrintf(db.ALWAYS, "[%v] WASM instantiation: %v", pid, time.Since(t_instantiate))

	// t_alloc := time.Now()
	// if err := inst.allocateSharedBuffer(); err != nil {
	// 	return err
	// }
	// db.DPrintf(db.ALWAYS, "[%v] Buffer allocation: %v", pid, time.Since(t_alloc))

	t_lookup := time.Now()
	entryFn, err := inst.instance.Exports.GetFunction("entrypoint")
	if err != nil {
		db.DPrintf(db.WASMRT_ERR, "[%v] No entry function found", pid)
		return fmt.Errorf("no entry function: %w", err)
	}
	db.DPrintf(db.ALWAYS, "[%v] Entry function lookup: %v", pid, time.Since(t_lookup))

	t_execute := time.Now()
	db.DPrintf(db.WASMRT, "[%v] Executing entry function", pid)
	result, err := entryFn()
	if err != nil {
		db.DPrintf(db.WASMRT_ERR, "[%v] Execution error: %v", pid, err)
		return fmt.Errorf("execute: %w", err)
	}
	perf.LogSpawnLatency("WASM execution", pid, inst.uproc.GetSpawnTime(), t_execute)
	db.DPrintf(db.ALWAYS, "[%v] WASM execution: %v", pid, time.Since(t_execute))

	db.DPrintf(db.ALWAYS, "[%v] Total WASM runtime: %v, result=%v", pid, time.Since(t_overall), result)
	db.DPrintf(db.WASMRT, "[%v] Completed successfully, result=%v", pid, result)
	return nil
}

func (inst *Instance) allocateSharedBuffer() error {
	allocFn, err := inst.instance.Exports.GetFunction("allocate")
	if err != nil {
		db.DPrintf(db.WASMRT, "[%v] No allocate function, skipping", inst.uproc.GetPid())
		return nil
	}

	memPtr, err := allocFn(SHARED_BUF_SZ)
	if err != nil {
		return fmt.Errorf("allocate buffer: %w", err)
	}

	inst.wasmBufPtr = memPtr.(int32)
	db.DPrintf(db.WASMRT, "[%v] Allocated buffer at %d", inst.uproc.GetPid(), inst.wasmBufPtr)
	return nil
}

func (inst *Instance) cleanup() {
	pid := inst.uproc.GetPid()
	db.DPrintf(db.WASMRT, "[%v] Cleaning up", pid)

	if inst.spClnt != nil {
		if err := inst.spClnt.Close(); err != nil {
			db.DPrintf(db.WASMRT_ERR, "[%v] Error closing sigmap client: %v", pid, err)
		}
	}
}
