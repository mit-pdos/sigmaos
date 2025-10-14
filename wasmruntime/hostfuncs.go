package wasmruntime

import (
	"time"

	wasmer "github.com/wasmerio/wasmer-go/wasmer"

	db "sigmaos/debug"
	"sigmaos/proc"
	sp "sigmaos/sigmap"
)

func (inst *Instance) createHostFunctions(wasiImports *wasmer.ImportObject) *wasmer.ImportObject {
	var importObject *wasmer.ImportObject
	if wasiImports != nil {
		importObject = wasiImports
	} else {
		importObject = wasmer.NewImportObject()
	}

	importObject.Register("env", map[string]wasmer.IntoExtern{
		"Started":    inst.newStartedFn(),
		"Exited":     inst.newExitedFn(),
		"Open":       inst.newOpenFn(),
		"Create":     inst.newCreateFn(),
		"Read":       inst.newReadFn(),
		"Write":      inst.newWriteFn(),
		"CloseFd":    inst.newCloseFdFn(),
		"GetArgs":    inst.newGetArgsFn(),
		"Log":        inst.newLogFn(),
		"GetArgc":    inst.newGetArgcFn(),
		"GetArgvLen": inst.newGetArgvLenFn(),
	})

	return importObject
}

func (inst *Instance) newStartedFn() *wasmer.Function {
	return wasmer.NewFunction(
		inst.store,
		wasmer.NewFunctionType(wasmer.NewValueTypes(), wasmer.NewValueTypes(wasmer.I32)),
		func(args []wasmer.Value) ([]wasmer.Value, error) {
			pid := inst.uproc.GetPid()
			now := time.Now()
			spawnLatency := now.Sub(inst.instantiateTime)

			db.DPrintf(db.ALWAYS, "[%v] Started() called from WASM - spawn latency (instantiate to Started): %v", pid, spawnLatency)
			db.DPrintf(db.WASMRT, "[%v] Started() called from WASM", pid)

			err := inst.spClnt.Started()
			if err != nil {
				db.DPrintf(db.WASMRT_ERR, "[%v] Started() failed: %v", pid, err)
				return []wasmer.Value{wasmer.NewI32(0)}, err
			}

			db.DPrintf(db.WASMRT, "[%v] Started() succeeded", pid)
			return []wasmer.Value{wasmer.NewI32(1)}, nil
		},
	)
}

func (inst *Instance) newExitedFn() *wasmer.Function {
	return wasmer.NewFunction(
		inst.store,
		wasmer.NewFunctionType(wasmer.NewValueTypes(wasmer.I32), wasmer.NewValueTypes()),
		func(args []wasmer.Value) ([]wasmer.Value, error) {
			pid := inst.uproc.GetPid()
			status := args[0].I32()
			db.DPrintf(db.WASMRT, "[%v] Exited(status=%d) called from WASM", pid, status)

			err := inst.spClnt.Exited(proc.Tstatus(status), "WASM module exited")
			if err != nil {
				db.DPrintf(db.WASMRT_ERR, "[%v] Exited() failed: %v", pid, err)
				return []wasmer.Value{}, err
			}

			db.DPrintf(db.WASMRT, "[%v] Exited() succeeded", pid)
			return []wasmer.Value{}, nil
		},
	)
}

// Open(path_ptr: i32, path_len: i32, mode: i32) -> fd: i32
func (inst *Instance) newOpenFn() *wasmer.Function {
	return wasmer.NewFunction(
		inst.store,
		wasmer.NewFunctionType(wasmer.NewValueTypes(wasmer.I32, wasmer.I32, wasmer.I32), wasmer.NewValueTypes(wasmer.I32)),
		func(args []wasmer.Value) ([]wasmer.Value, error) {
			pid := inst.uproc.GetPid()
			pathPtr := args[0].I32()
			pathLen := args[1].I32()
			mode := args[2].I32()

			path := inst.readString(pathPtr, pathLen)
			db.DPrintf(db.WASMRT, "[%v] Open(%s, mode=%d) called", pid, path, mode)

			fd, err := inst.spClnt.Open(path, sp.Tmode(mode), false)
			if err != nil {
				db.DPrintf(db.WASMRT_ERR, "[%v] Open() failed: %v", pid, err)
				return []wasmer.Value{wasmer.NewI32(-1)}, nil
			}

			db.DPrintf(db.WASMRT, "[%v] Open() returned fd=%d", pid, fd)
			return []wasmer.Value{wasmer.NewI32(int32(fd))}, nil
		},
	)
}

// Create(path_ptr: i32, path_len: i32, perm: i32, mode: i32) -> fd: i32
func (inst *Instance) newCreateFn() *wasmer.Function {
	return wasmer.NewFunction(
		inst.store,
		wasmer.NewFunctionType(wasmer.NewValueTypes(wasmer.I32, wasmer.I32, wasmer.I32, wasmer.I32), wasmer.NewValueTypes(wasmer.I32)),
		func(args []wasmer.Value) ([]wasmer.Value, error) {
			pid := inst.uproc.GetPid()
			pathPtr := args[0].I32()
			pathLen := args[1].I32()
			perm := args[2].I32()
			mode := args[3].I32()

			path := inst.readString(pathPtr, pathLen)
			db.DPrintf(db.WASMRT, "[%v] Create(%s, perm=%d, mode=%d) called", pid, path, perm, mode)

			fd, err := inst.spClnt.Create(path, sp.Tperm(perm), sp.Tmode(mode))
			if err != nil {
				db.DPrintf(db.WASMRT_ERR, "[%v] Create() failed: %v", pid, err)
				return []wasmer.Value{wasmer.NewI32(-1)}, nil
			}

			db.DPrintf(db.WASMRT, "[%v] Create() returned fd=%d", pid, fd)
			return []wasmer.Value{wasmer.NewI32(int32(fd))}, nil
		},
	)
}

// Read(fd: i32, buf_ptr: i32, buf_len: i32) -> bytes_read: i32
func (inst *Instance) newReadFn() *wasmer.Function {
	return wasmer.NewFunction(
		inst.store,
		wasmer.NewFunctionType(wasmer.NewValueTypes(wasmer.I32, wasmer.I32, wasmer.I32), wasmer.NewValueTypes(wasmer.I32)),
		func(args []wasmer.Value) ([]wasmer.Value, error) {
			pid := inst.uproc.GetPid()
			fd := args[0].I32()
			bufPtr := args[1].I32()
			bufLen := args[2].I32()

			db.DPrintf(db.WASMRT, "[%v] Read(fd=%d, len=%d) called", pid, fd, bufLen)

			buf := make([]byte, bufLen)
			n, err := inst.spClnt.Read(int(fd), buf)
			if err != nil {
				db.DPrintf(db.WASMRT_ERR, "[%v] Read() failed: %v", pid, err)
				return []wasmer.Value{wasmer.NewI32(-1)}, nil
			}

			inst.writeBytes(bufPtr, buf[:n])
			db.DPrintf(db.WASMRT, "[%v] Read() returned %d bytes", pid, n)
			return []wasmer.Value{wasmer.NewI32(int32(n))}, nil
		},
	)
}

// Write(fd: i32, buf_ptr: i32, buf_len: i32) -> bytes_written: i32
func (inst *Instance) newWriteFn() *wasmer.Function {
	return wasmer.NewFunction(
		inst.store,
		wasmer.NewFunctionType(wasmer.NewValueTypes(wasmer.I32, wasmer.I32, wasmer.I32), wasmer.NewValueTypes(wasmer.I32)),
		func(args []wasmer.Value) ([]wasmer.Value, error) {
			pid := inst.uproc.GetPid()
			fd := args[0].I32()
			bufPtr := args[1].I32()
			bufLen := args[2].I32()

			buf := inst.readBytes(bufPtr, bufLen)
			db.DPrintf(db.WASMRT, "[%v] Write(fd=%d, len=%d) called", pid, fd, bufLen)

			n, err := inst.spClnt.Write(int(fd), buf)
			if err != nil {
				db.DPrintf(db.WASMRT_ERR, "[%v] Write() failed: %v", pid, err)
				return []wasmer.Value{wasmer.NewI32(-1)}, nil
			}

			db.DPrintf(db.WASMRT, "[%v] Write() returned %d bytes", pid, n)
			return []wasmer.Value{wasmer.NewI32(int32(n))}, nil
		},
	)
}

// CloseFd(fd: i32) -> status: i32
func (inst *Instance) newCloseFdFn() *wasmer.Function {
	return wasmer.NewFunction(
		inst.store,
		wasmer.NewFunctionType(wasmer.NewValueTypes(wasmer.I32), wasmer.NewValueTypes(wasmer.I32)),
		func(args []wasmer.Value) ([]wasmer.Value, error) {
			pid := inst.uproc.GetPid()
			fd := args[0].I32()

			db.DPrintf(db.WASMRT, "[%v] CloseFd(fd=%d) called", pid, fd)

			err := inst.spClnt.CloseFd(int(fd))
			if err != nil {
				db.DPrintf(db.WASMRT_ERR, "[%v] CloseFd() failed: %v", pid, err)
				return []wasmer.Value{wasmer.NewI32(-1)}, nil
			}

			db.DPrintf(db.WASMRT, "[%v] CloseFd() succeeded", pid)
			return []wasmer.Value{wasmer.NewI32(0)}, nil
		},
	)
}

// GetArgc() -> argc: i32
func (inst *Instance) newGetArgcFn() *wasmer.Function {
	return wasmer.NewFunction(
		inst.store,
		wasmer.NewFunctionType(wasmer.NewValueTypes(), wasmer.NewValueTypes(wasmer.I32)),
		func(args []wasmer.Value) ([]wasmer.Value, error) {
			argc := len(inst.uproc.GetArgs())
			db.DPrintf(db.WASMRT, "[%v] GetArgc() returned %d", inst.uproc.GetPid(), argc)
			return []wasmer.Value{wasmer.NewI32(int32(argc))}, nil
		},
	)
}

// GetArgvLen(idx: i32) -> len: i32
func (inst *Instance) newGetArgvLenFn() *wasmer.Function {
	return wasmer.NewFunction(
		inst.store,
		wasmer.NewFunctionType(wasmer.NewValueTypes(wasmer.I32), wasmer.NewValueTypes(wasmer.I32)),
		func(args []wasmer.Value) ([]wasmer.Value, error) {
			idx := args[0].I32()
			procArgs := inst.uproc.GetArgs()
			if idx < 0 || int(idx) >= len(procArgs) {
				return []wasmer.Value{wasmer.NewI32(-1)}, nil
			}
			length := len(procArgs[idx])
			db.DPrintf(db.WASMRT, "[%v] GetArgvLen(%d) returned %d", inst.uproc.GetPid(), idx, length)
			return []wasmer.Value{wasmer.NewI32(int32(length))}, nil
		},
	)
}

// GetArgs(idx: i32, buf_ptr: i32, buf_len: i32) -> status: i32
func (inst *Instance) newGetArgsFn() *wasmer.Function {
	return wasmer.NewFunction(
		inst.store,
		wasmer.NewFunctionType(wasmer.NewValueTypes(wasmer.I32, wasmer.I32, wasmer.I32), wasmer.NewValueTypes(wasmer.I32)),
		func(args []wasmer.Value) ([]wasmer.Value, error) {
			pid := inst.uproc.GetPid()
			idx := args[0].I32()
			bufPtr := args[1].I32()
			bufLen := args[2].I32()

			procArgs := inst.uproc.GetArgs()
			if idx < 0 || int(idx) >= len(procArgs) {
				db.DPrintf(db.WASMRT_ERR, "[%v] GetArgs(%d) index out of bounds", pid, idx)
				return []wasmer.Value{wasmer.NewI32(-1)}, nil
			}

			arg := procArgs[idx]
			if int32(len(arg)) > bufLen {
				db.DPrintf(db.WASMRT_ERR, "[%v] GetArgs(%d) buffer too small", pid, idx)
				return []wasmer.Value{wasmer.NewI32(-1)}, nil
			}

			inst.writeBytes(bufPtr, []byte(arg))
			db.DPrintf(db.WASMRT, "[%v] GetArgs(%d) returned '%s'", pid, idx, arg)
			return []wasmer.Value{wasmer.NewI32(0)}, nil
		},
	)
}

// Log(msg_ptr: i32, msg_len: i32)
func (inst *Instance) newLogFn() *wasmer.Function {
	return wasmer.NewFunction(
		inst.store,
		wasmer.NewFunctionType(wasmer.NewValueTypes(wasmer.I32, wasmer.I32), wasmer.NewValueTypes()),
		func(args []wasmer.Value) ([]wasmer.Value, error) {
			msgPtr := args[0].I32()
			msgLen := args[1].I32()

			msg := inst.readString(msgPtr, msgLen)
			db.DPrintf(db.ALWAYS, "[WASM %v] %s", inst.uproc.GetPid(), msg)
			return []wasmer.Value{}, nil
		},
	)
}

func (inst *Instance) readBytes(ptr, length int32) []byte {
	memory, _ := inst.instance.Exports.GetMemory("memory")
	data := memory.Data()[ptr : ptr+length]
	result := make([]byte, length)
	copy(result, data)
	return result
}

func (inst *Instance) readString(ptr, length int32) string {
	return string(inst.readBytes(ptr, length))
}

func (inst *Instance) writeBytes(ptr int32, data []byte) {
	memory, _ := inst.instance.Exports.GetMemory("memory")
	copy(memory.Data()[ptr:], data)
}
