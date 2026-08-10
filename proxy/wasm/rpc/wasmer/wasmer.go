package wasmer

import (
	"bytes"
	"encoding/binary"
	"time"

	wasmer "github.com/wasmerio/wasmer-go/wasmer"

	db "sigmaos/debug"
	wasmrpc "sigmaos/proxy/wasm/rpc"
	sp "sigmaos/sigmap"
	"sigmaos/util/perf"
)

const (
	DEFAULT_WASM_BUF_SZ = 20 * sp.MBYTE
)

type WasmerRuntime struct {
	apiImpl         wasmrpc.CoSandboxAPIImpl
	recvDelegatedFn func(uint64) ([]byte, error) // nil in spproxy path
	precompStore    *wasmer.Store                 // Store used for WASM script precompilation
	bufSz           int32                         // shared buffer size for current RunModule call
}

func NewWasmerRuntime(apiImpl wasmrpc.CoSandboxAPIImpl) *WasmerRuntime {
	// TODO: get LLVM compiler to work, since it produces faster (and smaller)
	// binaries
	// TODO: try this https://github.com/wasmerio/wasmer-go/issues/222
	//	cfg := wasmer.NewConfig().UseLLVMCompiler()
	cfg := wasmer.NewConfig().UseCraneliftCompiler()
	engine := wasmer.NewEngineWithConfig(cfg)
	return &WasmerRuntime{
		apiImpl:      apiImpl,
		precompStore: wasmer.NewStore(engine),
	}
}

func (wrt *WasmerRuntime) SetRecvDelegated(fn func(uint64) ([]byte, error)) {
	wrt.recvDelegatedFn = fn
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

// RunModule runs a co-sandbox from bytes carried inline in the proc.
//
// Those bytes are *precompiled* — PrecompileModule output, produced by whoever
// called ReadCoSandbox — so this deserializes and never compiles. Do not pass a
// raw .wasm here; it would fail deserialization rather than being compiled.
// RunModulePath is the one that takes a .wasm.
//
// Each call still deserializes into a store of its own, which is why a proc
// spawned many times over should name its co-sandbox instead.
func (wrt *WasmerRuntime) RunModule(pid sp.Tpid, spawnTime time.Time, compiledModule []byte, inputBytes []byte, bufSz int) (wasmrpc.Tstatus, string, error) {
	engine := wasmer.NewEngine()
	store := wasmer.NewStore(engine)
	module, err := wasmer.DeserializeModule(store, compiledModule)
	if err != nil {
		db.DPrintf(db.ERROR, "[%v] Err in compiled WASM module deserialization: %v", pid, err)
		db.DPrintf(db.WASMRT_ERR, "[%v] Err in compiled WASM module deserialization: %v", pid, err)
		return wasmrpc.EXIT_ERR, sp.NOT_SET, err
	}
	db.DPrintf(db.WASMRT, "Deserialized compiled WASM module")
	// Not entered into the module cache: these bytes came with one proc and are
	// keyed by nothing. The wrapper is just to share runModule's body, and its
	// lock is uncontended.
	return wrt.runModule(pid, spawnTime, newCachedModule(store, module), inputBytes, bufSz)
}

// RunModulePath runs the co-sandbox binary at the local pathname pn, compiling
// it on first use and reusing the compiled module for every later proc on this
// node. pn is a .wasm as uploaded, not the precompiled form RunModule takes:
// the compilation this saves is the whole point of the cache.
func (wrt *WasmerRuntime) RunModulePath(pid sp.Tpid, spawnTime time.Time, pn string, inputBytes []byte, bufSz int) (wasmrpc.Tstatus, string, error) {
	cm, err := loadModule(pid, pn)
	if err != nil {
		return wasmrpc.EXIT_ERR, sp.NOT_SET, err
	}
	return wrt.runModule(pid, spawnTime, cm, inputBytes, bufSz)
}

// runModule instantiates cm and runs its boot function. The import functions
// must be created with cm's store, since wasmer requires an instance's imports
// and module to share one.
func (wrt *WasmerRuntime) runModule(pid sp.Tpid, spawnTime time.Time, cm *cachedModule, inputBytes []byte, bufSz int) (wasmrpc.Tstatus, string, error) {
	wrt.bufSz = int32(bufSz)
	store, module := cm.store, cm.module
	// Instantiation touches the shared store, which wasmer does not promise is
	// safe to use from several goroutines at once. Held only across the
	// instantiation, not across the boot call, which is the long part.
	cm.mu.Lock()
	defer cm.mu.Unlock()
	var buf []byte
	var instance *wasmer.Instance
	var wasmBufPtr int32
	// Register SigmaOS host API calls
	importObject := wasmer.NewImportObject()
	importObject.Register(
		"sigmaos_host",
		map[string]wasmer.IntoExtern{
			"send_rpc":           wrt.newSendRPCFn(store, &instance, &wasmBufPtr, pid),
			"recv_rpc":           wrt.newRecvRPCFn(store, &instance, &wasmBufPtr, pid),
			"recv_delegated_rpc": wrt.newRecvDelegatedRPCFn(store, &instance, &wasmBufPtr, pid),
			"forward_rpc":        wrt.newForwardRPCFn(store, &instance, &wasmBufPtr, pid),
			"exit":                wrt.newExitFn(store, &instance, &wasmBufPtr, pid),
			"log":                 wrt.newLogFn(store, &instance, &wasmBufPtr, pid),
			"log_spawn_latency":   wrt.newLogSpawnLatencyFn(store, &instance, &wasmBufPtr, pid),
			"get_run_co_sandbox":  wrt.newGetRunCoSandboxFn(store, pid),
			"get_time_us":         wrt.newGetTimeUsFn(store, pid),
		},
	)
	start := time.Now()
	// Instantiate the module
	instance, err := wasmer.NewInstance(module, importObject)
	if err != nil {
		db.DPrintf(db.ERROR, "[%v] Err instantiate WASM module: %v", pid, err)
		db.DPrintf(db.WASMRT_ERR, "[%v] Err instantiate WASM module: %v", pid, err)
		return wasmrpc.EXIT_ERR, sp.NOT_SET, err
	}
	perf.LogSpawnLatency("WASM module instantiation", pid, perf.TIME_NOT_SET, start)
	// Get a function pointer to the module's allocate function, which the
	// runtime uses to allocate a shared buffer in the WASM module's memory
	allocFn, err := instance.Exports.GetFunction("allocate")
	if err != nil {
		db.DPrintf(db.ERROR, "[%v] Err get WASM module allocate fn: %v", pid, err)
		db.DPrintf(db.WASMRT_ERR, "[%v] Err get WASM module allocate fn: %v", pid, err)
		return wasmrpc.EXIT_ERR, sp.NOT_SET, err
	}
	memPtr, err := allocFn(wrt.bufSz)
	if err != nil {
		db.DPrintf(db.ERROR, "[%v] Err allocate shared buffer with WASM module: %v", pid, err)
		db.DPrintf(db.WASMRT_ERR, "[%v] Err allocate shared buffer with WASM module: %v", pid, err)
		return wasmrpc.EXIT_ERR, sp.NOT_SET, err
	}
	wasmBufPtr = memPtr.(int32)
	db.DPrintf(db.WASMRT, "[%v] WASM-allocated buffer address: %v", pid, wasmBufPtr)
	mem, err := instance.Exports.GetMemory("memory")
	if err != nil {
		db.DPrintf(db.ERROR, "[%v] Err get WASM instance memory: %v", pid, err)
		db.DPrintf(db.WASMRT_ERR, "[%v] Err get WASM instance memory: %v", pid, err)
		return wasmrpc.EXIT_ERR, sp.NOT_SET, err
	}
	// Create a Go buffer from the allocated WASM shared buffer
	buf = mem.Data()[wasmBufPtr : wasmBufPtr+wrt.bufSz]
	// Copy the input bytes to the buffer
	copy(buf, inputBytes)
	// Get a function pointer to the "boot" function exposed by the module
	boot, err := instance.Exports.GetFunction("boot")
	if err != nil {
		db.DPrintf(db.ERROR, "[%v] Err get WASM boot function: %v", pid, err)
		db.DPrintf(db.WASMRT_ERR, "[%v] Err get WASM boot function: %v", pid, err)
		return wasmrpc.EXIT_ERR, sp.NOT_SET, err
	}
	start = time.Now()
	// Call the boot function and inform it of the size & address of the shared
	// buffer.
	if _, err := boot(wasmBufPtr, wrt.bufSz); err != nil {
		db.DPrintf(db.ERROR, "[%v] Err run WASM function: %v", pid, err)
		db.DPrintf(db.WASMRT_ERR, "[%v] Err run WASM function: %v", pid, err)
		return wasmrpc.EXIT_ERR, sp.NOT_SET, err
	}
	perf.LogSpawnLatency("WASM module ran", pid, spawnTime, start)
	db.DPrintf(db.WASMRT, "[%v] Successfully ran WASM boot script", pid)
	return wrt.apiImpl.WaitExit()
}

func (wrt *WasmerRuntime) newRecvDelegatedRPCFn(store *wasmer.Store, instance **wasmer.Instance, wasmBufPtr *int32, pid sp.Tpid) *wasmer.Function {
	return wasmer.NewFunction(
		store,
		wasmer.NewFunctionType(wasmer.NewValueTypes(wasmer.I64), wasmer.NewValueTypes(wasmer.I64)),
		func(args []wasmer.Value) ([]wasmer.Value, error) {
			rpcIdx := uint64(args[0].I64())
			if wrt.recvDelegatedFn == nil {
				db.DFatalf("[%v] recv_delegated_rpc called with nil recvDelegatedFn", pid)
			}
			data, err := wrt.recvDelegatedFn(rpcIdx)
			if err != nil {
				db.DPrintf(db.WASMRT_ERR, "[%v] Err RecvDelegatedRPC(%v): %v", pid, rpcIdx, err)
				return []wasmer.Value{}, err
			}
			mem, err := (*instance).Exports.GetMemory("memory")
			if err != nil {
				db.DPrintf(db.ERROR, "[%v] Err get WASM instance memory: %v", pid, err)
				return []wasmer.Value{}, err
			}
			buf := (*mem).Data()[*wasmBufPtr : *wasmBufPtr+wrt.bufSz]
			frameLen := uint64(len(data) + 8)
			binary.LittleEndian.PutUint64(buf[0:8], frameLen)
			copy(buf[8:], data)
			db.DPrintf(db.WASMRT, "[%v] RecvDelegatedRPC(%v) data len:%v", pid, rpcIdx, len(data))
			return []wasmer.Value{wasmer.NewI64(1)}, nil
		},
	)
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
			buf := (*mem).Data()[*wasmBufPtr : *wasmBufPtr+wrt.bufSz]
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
			err = wrt.apiImpl.Send(rpcIdx, pn, method, rpcBytes, nOutIOV)
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
		wasmer.NewFunctionType(wasmer.NewValueTypes(wasmer.I64, wasmer.I64), wasmer.NewValueTypes(wasmer.I64)),
		func(args []wasmer.Value) ([]wasmer.Value, error) {
			// Get the RPC index ID
			rpcIdx := uint64(args[0].I64())
			getData := args[1].I64() > 0
			db.DPrintf(db.WASMRT, "RecvRPC(%v)", rpcIdx)
			// Receive the RPC reply
			replyIOV, err := wrt.apiImpl.Recv(rpcIdx, getData)
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
			replyLen := 0
			// Only copy the data back to WASM if the module asks for it
			if getData {
				// Create a Go buffer from the allocated WASM shared buffer
				buf := (*mem).Data()[*wasmBufPtr : *wasmBufPtr+wrt.bufSz]
				//			return outiov.GetFrame(1).GetBuf(), nil
				bufIdx := 0
				// Skip index 0, which is the RPC stack wrapper
				for i := 1; i < replyIOV.Len(); i++ {
					frame := replyIOV.GetFrame(i)
					frameLen := uint64(frame.Len() + 8)
					var b bytes.Buffer
					if err := binary.Write(&b, binary.LittleEndian, frameLen); err != nil {
						db.DFatalf("Err encode buf len: %v", err)
					}
					copy(buf[bufIdx:bufIdx+8], b.Bytes())
					if uint64(bufIdx)+frameLen > uint64(wrt.bufSz) {
						db.DFatalf("Err copy too much data to WASM reply buffer: %v > %v", uint64(bufIdx)+frameLen, wrt.bufSz)
					}
					// Copy the reply to the shared buffer
					copy(buf[bufIdx+8:], frame.GetBuf())
					bufIdx += int(frameLen)
				}
				// Report the RPC reply's length back to the WASM module
				replyLen = replyIOV.Len() - 1
			}
			db.DPrintf(db.WASMRT, "RecvRPC(%v) reply len: %v", rpcIdx, replyLen)
			return []wasmer.Value{wasmer.NewI64(replyLen)}, nil
		},
	)
}

func (wrt *WasmerRuntime) newForwardRPCFn(store *wasmer.Store, instance **wasmer.Instance, wasmBufPtr *int32, pid sp.Tpid) *wasmer.Function {
	return wasmer.NewFunction(
		store,
		wasmer.NewFunctionType(wasmer.NewValueTypes(wasmer.I64, wasmer.I64, wasmer.I64, wasmer.I64), wasmer.NewValueTypes()),
		func(args []wasmer.Value) ([]wasmer.Value, error) {
			// Get the RPC index ID
			rpcIdx := uint64(args[0].I64())
			newRPCIdx := uint64(args[1].I64())
			pnLen := args[2].I64()
			nOutIOV := uint64(args[3].I64())
			mem, err := (*instance).Exports.GetMemory("memory")
			if err != nil {
				db.DPrintf(db.ERROR, "[%v] Err get WASM instance memory: %v", pid, err)
				db.DPrintf(db.WASMRT_ERR, "[%v] Err get WASM instance memory: %v", pid, err)
				return []wasmer.Value{}, err
			}
			// Create a Go buffer from the allocated WASM shared buffer
			buf := (*mem).Data()[*wasmBufPtr : *wasmBufPtr+wrt.bufSz]
			db.DPrintf(db.WASMRT, "rpcIdx: %v pnLen:%v buf:%p bufStart:%p", rpcIdx, pnLen, buf, &buf[0])
			// Get the RPC destination pathname from the shared buffer
			pn := string(buf[0:pnLen])
			db.DPrintf(db.WASMRT, "ForwardRPC(%v->%v) pn:%v nOutIOV:%v", rpcIdx, newRPCIdx, pn, nOutIOV)
			err = wrt.apiImpl.Forward(rpcIdx, newRPCIdx, pn, nOutIOV)
			if err != nil {
				db.DPrintf(db.WASMRT_ERR, "Err ForwardRPC(%v->%v): %v", rpcIdx, newRPCIdx, err)
				return []wasmer.Value{}, err
			}
			db.DPrintf(db.WASMRT, "ForwardRPC(%v->%v) done", rpcIdx, newRPCIdx)
			return []wasmer.Value{}, nil
		},
	)
}

func (wrt *WasmerRuntime) newExitFn(store *wasmer.Store, instance **wasmer.Instance, wasmBufPtr *int32, pid sp.Tpid) *wasmer.Function {
	return wasmer.NewFunction(
		store,
		wasmer.NewFunctionType(wasmer.NewValueTypes(wasmer.I64, wasmer.I64), wasmer.NewValueTypes()),
		func(args []wasmer.Value) ([]wasmer.Value, error) {
			// Get the RPC index ID
			status := wasmrpc.Tstatus(args[0].I64())
			msgLen := uint64(args[1].I64())
			mem, err := (*instance).Exports.GetMemory("memory")
			if err != nil {
				db.DPrintf(db.ERROR, "[%v] Err get WASM instance memory: %v", pid, err)
				db.DPrintf(db.WASMRT_ERR, "[%v] Err get WASM instance memory: %v", pid, err)
				return []wasmer.Value{}, err
			}
			// Create a Go buffer from the allocated WASM shared buffer
			buf := (*mem).Data()[*wasmBufPtr : *wasmBufPtr+wrt.bufSz]
			db.DPrintf(db.WASMRT, "exit status: %v msgLen:%v buf:%p bufStart:%p", status, msgLen, buf, &buf[0])
			// Get the RPC destination pathname from the shared buffer
			msg := string(buf[0:msgLen])
			db.DPrintf(db.WASMRT, "Exit status:%v msg:%v", status, msg)
			err = wrt.apiImpl.Exit(status, msg)
			if err != nil {
				db.DPrintf(db.WASMRT_ERR, "Err Exit status:%v msg:%v err:%v", status, msg, err)
				return []wasmer.Value{}, err
			}
			db.DPrintf(db.WASMRT, "Exit done status:%v msg:%v", status, msg)
			return []wasmer.Value{}, nil
		},
	)
}

func (wrt *WasmerRuntime) newLogFn(store *wasmer.Store, instance **wasmer.Instance, wasmBufPtr *int32, pid sp.Tpid) *wasmer.Function {
	return wasmer.NewFunction(
		store,
		wasmer.NewFunctionType(wasmer.NewValueTypes(wasmer.I64), wasmer.NewValueTypes()),
		func(args []wasmer.Value) ([]wasmer.Value, error) {
			msgLen := uint64(args[0].I64())
			mem, err := (*instance).Exports.GetMemory("memory")
			if err != nil {
				db.DPrintf(db.ERROR, "[%v] Err get WASM instance memory: %v", pid, err)
				db.DPrintf(db.WASMRT_ERR, "[%v] Err get WASM instance memory: %v", pid, err)
				return []wasmer.Value{}, err
			}
			buf := (*mem).Data()[*wasmBufPtr : *wasmBufPtr+wrt.bufSz]
			msg := string(buf[0:msgLen])
			if err := wrt.apiImpl.Log(msg); err != nil {
				db.DPrintf(db.WASMRT_ERR, "[%v] Err Log: %v", pid, err)
				return []wasmer.Value{}, err
			}
			return []wasmer.Value{}, nil
		},
	)
}

func (wrt *WasmerRuntime) newLogSpawnLatencyFn(store *wasmer.Store, instance **wasmer.Instance, wasmBufPtr *int32, pid sp.Tpid) *wasmer.Function {
	return wasmer.NewFunction(
		store,
		wasmer.NewFunctionType(wasmer.NewValueTypes(wasmer.I64, wasmer.I64), wasmer.NewValueTypes()),
		func(args []wasmer.Value) ([]wasmer.Value, error) {
			labelLen := uint64(args[0].I64())
			elapsedMicros := uint64(args[1].I64())
			mem, err := (*instance).Exports.GetMemory("memory")
			if err != nil {
				db.DPrintf(db.ERROR, "[%v] Err get WASM instance memory: %v", pid, err)
				db.DPrintf(db.WASMRT_ERR, "[%v] Err get WASM instance memory: %v", pid, err)
				return []wasmer.Value{}, err
			}
			buf := (*mem).Data()[*wasmBufPtr : *wasmBufPtr+wrt.bufSz]
			label := string(buf[0:labelLen])
			if err := wrt.apiImpl.LogSpawnLatency(label, elapsedMicros); err != nil {
				db.DPrintf(db.WASMRT_ERR, "[%v] Err LogSpawnLatency: %v", pid, err)
				return []wasmer.Value{}, err
			}
			return []wasmer.Value{}, nil
		},
	)
}

func (wrt *WasmerRuntime) newGetRunCoSandboxFn(store *wasmer.Store, pid sp.Tpid) *wasmer.Function {
	return wasmer.NewFunction(
		store,
		wasmer.NewFunctionType(wasmer.NewValueTypes(), wasmer.NewValueTypes(wasmer.I64)),
		func(args []wasmer.Value) ([]wasmer.Value, error) {
			val := int64(0)
			if wrt.apiImpl != nil && wrt.apiImpl.GetRunCoSandbox() {
				val = 1
			}
			db.DPrintf(db.WASMRT, "[%v] GetRunCoSandbox: %v", pid, val)
			return []wasmer.Value{wasmer.NewI64(val)}, nil
		},
	)
}

func (wrt *WasmerRuntime) newGetTimeUsFn(store *wasmer.Store, pid sp.Tpid) *wasmer.Function {
	return wasmer.NewFunction(
		store,
		wasmer.NewFunctionType(wasmer.NewValueTypes(), wasmer.NewValueTypes(wasmer.I64)),
		func(args []wasmer.Value) ([]wasmer.Value, error) {
			us := time.Now().UnixMicro()
			db.DPrintf(db.WASMRT, "[%v] GetTimeUs: %v", pid, us)
			return []wasmer.Value{wasmer.NewI64(us)}, nil
		},
	)
}
