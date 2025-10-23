package wasmer

import (
	"time"

	wasmer "github.com/wasmerio/wasmer-go/wasmer"

	db "sigmaos/debug"
	wasmrpc "sigmaos/proxy/wasm/rpc"
	sp "sigmaos/sigmap"
	"sigmaos/util/perf"
)

const (
	SHARED_BUF_SZ = 655360
)

type WasmerRuntime struct {
	rpcAPI       wasmrpc.RPCAPI
	precompStore *wasmer.Store // Store used for WASM script precompilation
}

func NewWasmerRuntime(rpcAPI wasmrpc.RPCAPI) *WasmerRuntime {
	// TODO: get LLVM compiler to work, since it produces faster (and smaller)
	// binaries
	// TODO: try this https://github.com/wasmerio/wasmer-go/issues/222
	//	cfg := wasmer.NewConfig().UseLLVMCompiler()
	var cfg *wasmer.Config
	if !wasmer.IsCompilerAvailable(wasmer.LLVM) {
		db.DPrintf(db.ERROR, "LLVM compiler not available, using Cranelift compiler instead")
		db.DPrintf(db.WASMRT_ERR, "LLVM compiler not available, using Cranelift compiler instead")
		cfg = wasmer.NewConfig().UseCraneliftCompiler()
	} else {
		db.DPrintf(db.WASMRT, "Using LLVM compiler")
		cfg = wasmer.NewConfig().UseLLVMCompiler()
	}
	engine := wasmer.NewEngineWithConfig(cfg)
	return &WasmerRuntime{
		rpcAPI:       rpcAPI,
		precompStore: wasmer.NewStore(engine),
	}
}

func (wrt *WasmerRuntime) PrecompileModule(wasmBytes []byte) ([]byte, error) {
	start := time.Now()
	module, err := wasmer.NewModule(wrt.precompStore, wasmBytes)
	if err != nil {
		db.DPrintf(db.ERROR, "Err in WASM module compilation: %v", err)
		db.DPrintf(db.WASMRT_ERR, "Err in WASM module compilation: %v", err)
		return nil, err
	}
	compiledModule, err := module.Serialize()
	if err != nil {
		db.DPrintf(db.ERROR, "Err in WASM module serialization: %v", err)
		db.DPrintf(db.WASMRT_ERR, "Err in WASM module serialization: %v", err)
		return nil, err
	}
	perf.LogSpawnLatency("WASM module compilation (%vB -> %vB)", sp.NOT_SET, perf.TIME_NOT_SET, start, len(wasmBytes), len(compiledModule))
	return compiledModule, nil
}

func (wrt *WasmerRuntime) RunModule(pid sp.Tpid, spawnTime time.Time, compiledModule []byte, inputBytes []byte) error {
	engine := wasmer.NewEngine()
	store := wasmer.NewStore(engine)
	module, err := wasmer.DeserializeModule(store, compiledModule)
	if err != nil {
		db.DPrintf(db.ERROR, "[%v] Err in compiled WASM module deserialization: %v", pid, err)
		db.DPrintf(db.WASMRT_ERR, "[%v] Err in compiled WASM module deserialization: %v", pid, err)
		return err
	}
	db.DPrintf(db.WASMRT, "Deserialized compiled WASM module")
	var buf []byte
	var instance *wasmer.Instance
	var wasmBufPtr int32
	// Register SigmaOS host API calls
	importObject := wasmer.NewImportObject()
	importObject.Register(
		"sigmaos_host",
		map[string]wasmer.IntoExtern{
			"send_rpc": wrt.newSendRPCFn(store, &instance, &wasmBufPtr, pid),
			"recv_rpc": wrt.newRecvRPCFn(store, &instance, &wasmBufPtr, pid),
		},
	)
	start := time.Now()
	// Instantiate the module
	instance, err = wasmer.NewInstance(module, importObject)
	if err != nil {
		db.DPrintf(db.ERROR, "[%v] Err instantiate WASM module: %v", pid, err)
		db.DPrintf(db.WASMRT_ERR, "[%v] Err instantiate WASM module: %v", pid, err)
		return err
	}
	perf.LogSpawnLatency("WASM module instantiation", pid, perf.TIME_NOT_SET, start)
	// Get a function pointer to the module's allocate function, which the
	// runtime uses to allocate a shared buffer in the WASM module's memory
	allocFn, err := instance.Exports.GetFunction("allocate")
	if err != nil {
		db.DPrintf(db.ERROR, "[%v] Err get WASM module allocate fn: %v", pid, err)
		db.DPrintf(db.WASMRT_ERR, "[%v] Err get WASM module allocate fn: %v", pid, err)
		return err
	}
	memPtr, err := allocFn(SHARED_BUF_SZ)
	if err != nil {
		db.DPrintf(db.ERROR, "[%v] Err allocate shared buffer with WASM module: %v", pid, err)
		db.DPrintf(db.WASMRT_ERR, "[%v] Err allocate shared buffer with WASM module: %v", pid, err)
		return err
	}
	wasmBufPtr = memPtr.(int32)
	db.DPrintf(db.WASMRT, "[%v] WASM-allocated buffer address: %v", pid, wasmBufPtr)
	mem, err := instance.Exports.GetMemory("memory")
	if err != nil {
		db.DPrintf(db.ERROR, "[%v] Err get WASM instance memory: %v", pid, err)
		db.DPrintf(db.WASMRT_ERR, "[%v] Err get WASM instance memory: %v", pid, err)
		return err
	}
	// Create a Go buffer from the allocated WASM shared buffer
	buf = mem.Data()[wasmBufPtr : wasmBufPtr+SHARED_BUF_SZ]
	// Copy the input bytes to the buffer
	copy(buf, inputBytes)
	// Get a function pointer to the "boot" function exposed by the module
	boot, err := instance.Exports.GetFunction("boot")
	if err != nil {
		db.DPrintf(db.ERROR, "[%v] Err get WASM boot function: %v", pid, err)
		db.DPrintf(db.WASMRT_ERR, "[%v] Err get WASM boot function: %v", pid, err)
		return err
	}
	start = time.Now()
	// Call the boot function and inform it of the size & address of the shared
	// buffer.
	if _, err := boot(wasmBufPtr, SHARED_BUF_SZ); err != nil {
		db.DPrintf(db.ERROR, "[%v] Err get WASM instance memory: %v", pid, err)
		db.DPrintf(db.WASMRT_ERR, "[%v] Err get WASM instance memory: %v", pid, err)
		return err
	}
	perf.LogSpawnLatency("WASM module ran", pid, spawnTime, start)
	db.DPrintf(db.WASMRT, "[%v] Successfully ran WASM boot script", pid)
	return nil
}

func (wrt *WasmerRuntime) newSendRPCFn(store *wasmer.Store, instance **wasmer.Instance, wasmBufPtr *int32, pid sp.Tpid) *wasmer.Function {
	return wasmer.NewFunction(
		store,
		wasmer.NewFunctionType(wasmer.NewValueTypes(wasmer.I64, wasmer.I64, wasmer.I64, wasmer.I64, wasmer.I64), wasmer.NewValueTypes()),
		func(args []wasmer.Value) ([]wasmer.Value, error) {
			// Get the RPC index ID
			rpcIdx := uint64(args[0].I64())
			pnLen := args[1].I64()
			methodLen := args[2].I64()
			rpcLen := args[3].I64()
			nOutIOV := uint64(args[4].I64())
			idx := int64(0)
			mem, err := (*instance).Exports.GetMemory("memory")
			if err != nil {
				db.DPrintf(db.ERROR, "[%v] Err get WASM instance memory: %v", pid, err)
				db.DPrintf(db.WASMRT_ERR, "[%v] Err get WASM instance memory: %v", pid, err)
				return []wasmer.Value{}, err
			}
			// Create a Go buffer from the allocated WASM shared buffer
			buf := (*mem).Data()[*wasmBufPtr : *wasmBufPtr+SHARED_BUF_SZ]
			db.DPrintf(db.WASMRT, "pnLen:%v methodLen:%v rpcLen:%v buf:%p bufStart:%p", pnLen, methodLen, rpcLen, buf, &buf[0])
			// Get the RPC destination pathname from the shared buffer
			pn := string(buf[idx : idx+pnLen])
			idx += pnLen
			// Get the method name from the shared buffer
			method := string(buf[idx : idx+methodLen])
			idx += methodLen
			// Get the marshaled RPC from the shared buffer
			rpcBytes := make([]byte, rpcLen)
			copy(rpcBytes, buf[idx:idx+rpcLen])
			db.DPrintf(db.WASMRT, "SendRPC(%v) pn:%v method:%v nbyte:%v", rpcIdx, pn, method, len(rpcBytes))
			err = wrt.rpcAPI.Send(rpcIdx, pn, method, rpcBytes, nOutIOV)
			if err != nil {
				db.DPrintf(db.WASMRT_ERR, "Err SendRPC(%v): %v", rpcIdx, err)
				return []wasmer.Value{}, err
			}
			db.DPrintf(db.WASMRT, "SendRPC(%v) done", rpcIdx)
			return []wasmer.Value{}, nil
		},
	)
}

func (wrt *WasmerRuntime) newRecvRPCFn(store *wasmer.Store, instance **wasmer.Instance, wasmBufPtr *int32, pid sp.Tpid) *wasmer.Function {
	return wasmer.NewFunction(
		store,
		wasmer.NewFunctionType(wasmer.NewValueTypes(wasmer.I64), wasmer.NewValueTypes(wasmer.I64)),
		func(args []wasmer.Value) ([]wasmer.Value, error) {
			// Get the RPC index ID
			rpcIdx := uint64(args[0].I64())
			db.DPrintf(db.WASMRT, "RecvRPC(%v)", rpcIdx)
			// Receive the RPC reply
			replyBytes, err := wrt.rpcAPI.Recv(rpcIdx)
			if err != nil {
				db.DPrintf(db.WASMRT_ERR, "Err RecvRPC(%v): %v", rpcIdx, err)
				return []wasmer.Value{}, err
			}
			mem, err := (*instance).Exports.GetMemory("memory")
			if err != nil {
				db.DPrintf(db.ERROR, "[%v] Err get WASM instance memory: %v", pid, err)
				db.DPrintf(db.WASMRT_ERR, "[%v] Err get WASM instance memory: %v", pid, err)
				return []wasmer.Value{}, err
			}
			// Create a Go buffer from the allocated WASM shared buffer
			buf := (*mem).Data()[*wasmBufPtr : *wasmBufPtr+SHARED_BUF_SZ]
			// Copy the reply to the shared buffer
			copy(buf, replyBytes)
			// Report the RPC reply's length back to the WASM module
			replyLen := len(replyBytes)
			db.DPrintf(db.WASMRT, "RecvRPC(%v) reply len: %v", rpcIdx, replyLen)
			return []wasmer.Value{wasmer.NewI64(replyLen)}, nil
		},
	)
}
