package wasmruntime

import (
	wasmer "github.com/wasmerio/wasmer-go/wasmer"

	db "sigmaos/debug"
	"sigmaos/proc"
)

func (inst *Instance) createHostFunctions(wasiImports *wasmer.ImportObject) *wasmer.ImportObject {
	var importObject *wasmer.ImportObject
	if wasiImports != nil {
		importObject = wasiImports
	} else {
		importObject = wasmer.NewImportObject()
	}

	importObject.Register("env", map[string]wasmer.IntoExtern{
		"Started": inst.newStartedFn(),
		"Exited":  inst.newExitedFn(),
	})

	return importObject
}

func (inst *Instance) newStartedFn() *wasmer.Function {
	return wasmer.NewFunction(
		inst.store,
		wasmer.NewFunctionType(wasmer.NewValueTypes(), wasmer.NewValueTypes(wasmer.I32)),
		func(args []wasmer.Value) ([]wasmer.Value, error) {
			pid := inst.uproc.GetPid()
			db.DPrintf(db.WASMRT, "[%v] Started() called from WASM", pid)

			err := inst.spClnt.Started()
			if err != nil {
				db.DPrintf(db.WASMRT_ERR, "[%v] Started() failed: %v", pid, err)
				return []wasmer.Value{wasmer.NewI32(0)}, err
			}

			db.DPrintf(db.WASMRT, "[%v] Started() succeeded", pid)
			return []wasmer.Value{wasmer.NewI32(123)}, nil
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
