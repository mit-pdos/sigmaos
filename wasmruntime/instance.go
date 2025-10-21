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

	t_store := time.Now()
	db.DPrintf(db.WASMRT, "[%v] Creating store from shared engine", pid)

	// Validate engine before creating store
	if inst.rt.engine == nil {
		db.DPrintf(db.WASMRT_ERR, "[%v] Runtime engine is nil", pid)
		return fmt.Errorf("runtime engine is nil")
	}
	db.DPrintf(db.WASMRT, "[%v] Engine pointer: %p", pid, inst.rt.engine)

	inst.store = wasmer.NewStore(inst.rt.engine)
	if inst.store == nil {
		db.DPrintf(db.WASMRT_ERR, "[%v] Failed to create store - returned nil", pid)
		return fmt.Errorf("store creation returned nil")
	}
	db.DPrintf(db.WASMRT, "[%v] Store created successfully, pointer: %p", pid, inst.store)
	db.DPrintf(db.ALWAYS, "[%v] Store creation: %v", pid, time.Since(t_store))

	t_compile := time.Now()
	db.DPrintf(db.WASMRT, "[%v] Compiling module (%d bytes)", pid, len(wasmBytes))
	module, err := wasmer.NewModule(inst.store, wasmBytes)
	if err != nil {
		db.DPrintf(db.WASMRT_ERR, "[%v] Compilation failed: %v", pid, err)
		return fmt.Errorf("compile module: %w", err)
	}
	if module == nil {
		db.DPrintf(db.WASMRT_ERR, "[%v] Module compilation returned nil without error", pid)
		return fmt.Errorf("module compilation returned nil")
	}
	db.DPrintf(db.WASMRT, "[%v] Module compiled successfully, pointer: %p", pid, module)
	perf.LogSpawnLatency("WASM compilation", pid, inst.uproc.GetSpawnTime(), t_compile)
	db.DPrintf(db.ALWAYS, "[%v] WASM compilation: %v", pid, time.Since(t_compile))

	t_wasi := time.Now()
	db.DPrintf(db.WASMRT, "[%v] Creating WASI environment", pid)
	db.DPrintf(db.WASMRT, "[%v] Building WASI state with name 'wasm-runtime'", pid)

	wasiBuilder := wasmer.NewWasiStateBuilder("wasm-runtime")
	if wasiBuilder == nil {
		db.DPrintf(db.WASMRT_ERR, "[%v] WASI state builder creation returned nil", pid)
		return fmt.Errorf("wasi state builder creation failed")
	}

	db.DPrintf(db.WASMRT, "[%v] Capturing stdout for WASI", pid)
	wasiBuilder = wasiBuilder.CaptureStdout()

	db.DPrintf(db.WASMRT, "[%v] Finalizing WASI environment (this may trigger LLVM initialization)", pid)
	wasiEnv, err := wasiBuilder.Finalize()
	if err != nil {
		db.DPrintf(db.WASMRT_ERR, "[%v] WASI env finalization failed: %v", pid, err)
		db.DPrintf(db.WASMRT_ERR, "[%v] Check if LLVM libraries are accessible in LD_LIBRARY_PATH", pid)
		return fmt.Errorf("create wasi env: %w", err)
	}
	if wasiEnv == nil {
		db.DPrintf(db.WASMRT_ERR, "[%v] WASI env creation returned nil without error", pid)
		return fmt.Errorf("wasi environment is nil")
	}
	db.DPrintf(db.WASMRT, "[%v] WASI environment created successfully, pointer: %p", pid, wasiEnv)
	db.DPrintf(db.ALWAYS, "[%v] WASI env creation: %v", pid, time.Since(t_wasi))

	t_imports := time.Now()
	db.DPrintf(db.WASMRT, "[%v] Generating WASI imports", pid)
	wasiImports, err := wasiEnv.GenerateImportObject(inst.store, module)
	if err != nil {
		db.DPrintf(db.WASMRT_ERR, "[%v] WASI import generation failed: %v", pid, err)
		return fmt.Errorf("generate wasi imports: %w", err)
	}
	if wasiImports == nil {
		db.DPrintf(db.WASMRT_ERR, "[%v] WASI imports object is nil", pid)
		return fmt.Errorf("wasi imports is nil")
	}
	db.DPrintf(db.WASMRT, "[%v] WASI imports generated successfully", pid)
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
