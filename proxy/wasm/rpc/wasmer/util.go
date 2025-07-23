package wasmer

import (
	"os"
	"path/filepath"
	"runtime"

	db "sigmaos/debug"
)

func projectRootPath() string {
	_, b, _, _ := runtime.Caller(0)
	return filepath.Dir(filepath.Dir(filepath.Dir(filepath.Dir(filepath.Dir(b)))))
}

func ReadBootScript(scriptName string) ([]byte, error) {
	// Compute WASM binary path name
	pn := filepath.Join(
		projectRootPath(),
		"rs/wasm",
		scriptName,
		"target/wasm32-unknown-unknown/release/",
		scriptName+".wasm",
	)
	db.DPrintf(db.ALWAYS, "Boot script path: %v", pn)
	b, err := os.ReadFile(pn)
	if err != nil {
		return nil, err
	}
	wrt := NewWasmerRuntime(nil)
	return wrt.PrecompileModule(b)
}
