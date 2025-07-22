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
	cfg := wasmer.NewConfig().UseCraneliftCompiler()
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

func (wrt *WasmerRuntime) RunModule(pid sp.Tpid, compiledModule []byte, inputBytes []byte) error {
	engine := wasmer.NewEngine()
	store := wasmer.NewStore(engine)
	module, err := wasmer.DeserializeModule(store, compiledModule)
	if err != nil {
		db.DPrintf(db.ERROR, "Err in compiled WASM module deserialization: %v", err)
		db.DPrintf(db.WASMRT_ERR, "Err in compiled WASM module deserialization: %v", err)
		return err
	}
	db.DPrintf(db.WASMRT, "Deserialized compiled WASM module")
	var buf []byte
	importObject := wasmer.NewImportObject()
	importObject.Register(
		"sigmaos_host",
		map[string]wasmer.IntoExtern{
			"send_rpc": wrt.newSendRPCFn(store, &buf),
			"recv_rpc": wrt.newRecvRPCFn(store, &buf),
		},
	)
	start := time.Now()
	instance, err := wasmer.NewInstance(module, importObject)
	if err != nil {
		db.DPrintf(db.ERROR, "Err instantiate WASM module: %v", err)
		db.DPrintf(db.WASMRT_ERR, "Err instantiate WASM module: %v", err)
		return err
	}
	perf.LogSpawnLatency("WASM module instantiation", pid, perf.TIME_NOT_SET, start)
	allocFn, err := instance.Exports.GetFunction("allocate")
	if err != nil {
		db.DPrintf(db.ERROR, "Err get WASM module allocate fn: %v", err)
		db.DPrintf(db.WASMRT_ERR, "Err get WASM module allocate fn: %v", err)
		return err
	}
	wasmBufPtr, err := allocFn(SHARED_BUF_SZ)
	if err != nil {
		db.DPrintf(db.ERROR, "Err allocate shared buffer with WASM module: %v", err)
		db.DPrintf(db.WASMRT_ERR, "Err allocate shared buffer with WASM module: %v", err)
		return err
	}
	db.DPrintf(db.TEST, "WASM-allocated buffer address: %v", wasmBufPtr)
	mem, err := instance.Exports.GetMemory("memory")
	if err != nil {
		db.DPrintf(db.ERROR, "Err get WASM instance memory: %v", err)
		db.DPrintf(db.WASMRT_ERR, "Err get WASM instance memory: %v", err)
		return err
	}
	buf = mem.Data()[wasmBufPtr.(int32) : wasmBufPtr.(int32)+SHARED_BUF_SZ]
	// Copy the input bytes to the buffer
	copy(buf, inputBytes)
	boot, err := instance.Exports.GetFunction("boot")
	if err != nil {
		db.DPrintf(db.ERROR, "Err get WASM boot function: %v", err)
		db.DPrintf(db.WASMRT_ERR, "Err get WASM boot function: %v", err)
		return err
	}
	start = time.Now()
	// Calls that exported function with Go standard values. The WebAssembly
	// types are inferred and values are casted automatically.
	if _, err := boot(wasmBufPtr, SHARED_BUF_SZ); err != nil {
		db.DPrintf(db.ERROR, "Err get WASM instance memory: %v", err)
		db.DPrintf(db.WASMRT_ERR, "Err get WASM instance memory: %v", err)
		return err
	}
	perf.LogSpawnLatency("WASM module ran", pid, perf.TIME_NOT_SET, start)
	db.DPrintf(db.WASMRT, "Successfully ran WASM module")
	return nil
}

func (wrt *WasmerRuntime) newSendRPCFn(store *wasmer.Store, buf *[]byte) *wasmer.Function {
	return wasmer.NewFunction(
		store,
		wasmer.NewFunctionType(wasmer.NewValueTypes(wasmer.I64, wasmer.I64, wasmer.I64), wasmer.NewValueTypes()),
		func(args []wasmer.Value) ([]wasmer.Value, error) {
			// Get the RPC index ID
			rpcIdx := uint64(args[0].I64())
			pn_len := args[1].I64()
			rpc_len := args[2].I64()
			// Get the RPC destination pathname from the shared buffer
			pn := string((*buf)[:pn_len])
			// Get the marshaled RPC from the shared buffer
			rpcBytes := (*buf)[pn_len : pn_len+rpc_len]
			db.DPrintf(db.WASMRT, "SendRPC(%v) pn:%v nbyte:%v", rpcIdx, pn, len(rpcBytes))
			err := wrt.rpcAPI.Send(rpcIdx, pn, rpcBytes)
			if err != nil {
				db.DPrintf(db.WASMRT_ERR, "Err SendRPC(%v): %v", rpcIdx, err)
				return []wasmer.Value{}, err
			}
			db.DPrintf(db.WASMRT, "SendRPC(%v) done", rpcIdx)
			return []wasmer.Value{}, nil
		},
	)
}

func (wrt *WasmerRuntime) newRecvRPCFn(store *wasmer.Store, buf *[]byte) *wasmer.Function {
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
			// Copy the reply to the shared buffer
			copy(*buf, replyBytes)
			// Report the RPC reply's length back to the WASM module
			replyLen := len(replyBytes)
			db.DPrintf(db.WASMRT, "RecvRPC(%v) reply len: %v", rpcIdx, replyLen)
			return []wasmer.Value{wasmer.NewI64(replyLen)}, nil
		},
	)
}
