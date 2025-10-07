// Package wasmruntime provides an in-process WASM runtime for SigmaOS
package wasmruntime

import (
	"fmt"
	"os"

	db "sigmaos/debug"
	"sigmaos/proc"
	sp "sigmaos/sigmap"
)

// Runtime manages WASM instances
type Runtime struct {
}

// WasmThread represents a WASM instance running as a thread
type WasmThread struct {
	uproc *proc.Proc
	pid   sp.Tpid
}

// NewRuntime creates a new WASM runtime
func NewRuntime() *Runtime {
	db.DPrintf(db.ALWAYS, "[wasmruntime] NewRuntime called")
	return &Runtime{}
}

// SpawnInstance creates and launches a new WASM instance
// For now, it just reads the wasm file and prints success
func (rt *Runtime) SpawnInstance(uproc *proc.Proc, wasmPath string) (*WasmThread, error) {
	db.DPrintf(db.ALWAYS, "[wasmruntime] SpawnInstance called for pid=%v path=%s", uproc.GetPid(), wasmPath)

	// Read the WASM file
	wasmBytes, err := os.ReadFile(wasmPath)
	if err != nil {
		db.DPrintf(db.ALWAYS, "[wasmruntime] Error reading wasm file %s: %v", wasmPath, err)
		return nil, fmt.Errorf("failed to read wasm file: %v", err)
	}

	db.DPrintf(db.ALWAYS, "[wasmruntime] Successfully read wasm file: %d bytes", len(wasmBytes))

	wt := &WasmThread{
		uproc: uproc,
		pid:   uproc.GetPid(),
	}

	return wt, nil
}

// Wait blocks until the WASM instance completes
func (wt *WasmThread) Wait() error {
	db.DPrintf(db.ALWAYS, "[wasmruntime] WasmThread.Wait called for pid=%v", wt.pid)
	// Minimal implementation - just return nil for now
	return nil
}

// Pid returns the process ID (returns 0 for WASM threads as they don't have OS PIDs)
func (wt *WasmThread) Pid() int {
	return 0
}
