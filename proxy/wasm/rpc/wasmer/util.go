package wasmer

import (
	"os"
	"path/filepath"
	"runtime"

	db "sigmaos/debug"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
)

func projectRootPath() string {
	_, b, _, _ := runtime.Caller(0)
	return filepath.Dir(filepath.Dir(filepath.Dir(filepath.Dir(filepath.Dir(b)))))
}

func ReadBootScript(sc *sigmaclnt.SigmaClnt, scriptName string) ([]byte, error) {
	var b []byte
	var err error
	// If this is a local build, get the script from the local filesystem
	if sc.ProcEnv().BuildTag == sp.LOCAL_BUILD {
		// Compute WASM binary path name
		pn := filepath.Join(
			projectRootPath(),
			"bin/wasm",
			scriptName+".wasm",
		)
		db.DPrintf(db.ALWAYS, "Boot script path: %v", pn)
		if b, err = os.ReadFile(pn); err != nil {
			db.DPrintf(db.ERROR, "Err read boot script local: %v", err)
			return nil, err
		}
	} else {
		// Else, read it out of S3
		pn := filepath.Join(sp.S3, sp.ANY, sc.ProcEnv().BuildTag, "wasm", scriptName+".wasm")
		if b, err = sc.GetFile(pn); err != nil {
			db.DPrintf(db.ERROR, "Err read boot script remote (%v): %v", pn, err)
			return nil, err
		}
	}
	wrt := NewWasmerRuntime(nil)
	return wrt.PrecompileModule(b)
}
