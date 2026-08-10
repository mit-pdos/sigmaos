package wasmer

import (
	"encoding/binary"
	"io"
	"os"
	"path/filepath"
	"runtime"

	db "sigmaos/debug"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
)

// EncodeArgs encodes a slice of strings as N u32 LE lengths followed by N
// string bodies — the format expected by WASM boot/proc input buffers.
func EncodeArgs(args []string) []byte {
	total := 4 * len(args)
	for _, a := range args {
		total += len(a)
	}
	buf := make([]byte, 0, total)
	for _, a := range args {
		var lb [4]byte
		binary.LittleEndian.PutUint32(lb[:], uint32(len(a)))
		buf = append(buf, lb[:]...)
	}
	for _, a := range args {
		buf = append(buf, a...)
	}
	return buf
}

func projectRootPath() string {
	_, b, _, _ := runtime.Caller(0)
	return filepath.Dir(filepath.Dir(filepath.Dir(filepath.Dir(filepath.Dir(b)))))
}

// CoSandboxProg is the filename of a co-sandbox binary, which is also the name
// procd hands chunksrv and therefore the key of procd's cache. upload.sh copies
// bin/wasm wholesale to s3://<buildTag>/wasm, so the name carries no build tag
// and the tag lives in the directory instead (CoSandboxDir).
//
// The consequence is that chunksrv's cache — keyed on the name alone,
// PathBinCache(realm, prog) — cannot tell two builds of the same co-sandbox
// apart. Harmless for a benchmark run, which boots a fresh cluster; on a
// long-lived cluster spanning two builds, a procd would keep serving the first
// one. Clear /tmp/sigmaos-bin/<kernelid> (or restart the kernel) if that ever
// bites.
func CoSandboxProg(scriptName string) string {
	return scriptName + ".wasm"
}

// CoSandboxDir is the directory holding a build's co-sandbox binaries, which is
// what procd passes chunksrv as the (single-element) search path.
func CoSandboxDir(buildTag string) string {
	return filepath.Join(sp.S3, sp.ANY, buildTag, "wasm")
}

// CoSandboxPath is the full sigma pathname of a co-sandbox binary: what goes in
// the proc, and what ReadCoSandboxRemote reads directly.
func CoSandboxPath(buildTag, scriptName string) string {
	return filepath.Join(CoSandboxDir(buildTag), CoSandboxProg(scriptName))
}

func UploadCoSandboxRemote(sc *sigmaclnt.SigmaClnt, scriptName string) error {
	pn := filepath.Join(
		projectRootPath(),
		"bin/wasm",
		scriptName+".wasm",
	)
	db.DPrintf(db.ALWAYS, "CoSandbox path: %v", pn)
	pnRemote := CoSandboxPath(sc.ProcEnv().BuildTag, scriptName)
	if err := sc.UploadFile(pn, pnRemote); err != nil {
		db.DPrintf(db.ERROR, "Err upload boot script (%v -> %v): %v", pn, pnRemote, err)
		return err
	}
	return nil
}

func ReadCoSandboxRemote(sc *sigmaclnt.SigmaClnt, scriptName string) ([]byte, error) {
	pn := CoSandboxPath(sc.ProcEnv().BuildTag, scriptName)
	rdr, err := sc.OpenReader(pn)
	if err != nil {
		db.DPrintf(db.ERROR, "Err open boot script remote (%v): %v", pn, err)
		return nil, err
	}
	defer rdr.Close()
	b, err := io.ReadAll(rdr)
	if err != nil {
		db.DPrintf(db.ERROR, "Err read boot script remote (%v): %v", pn, err)
		return nil, err
	}
	wrt := NewWasmerRuntime(nil)
	return wrt.PrecompileModule(b)
}

func ReadCoSandbox(sc *sigmaclnt.SigmaClnt, scriptName string) ([]byte, error) {
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
		db.DPrintf(db.ALWAYS, "CoSandbox path: %v", pn)
		if b, err = os.ReadFile(pn); err != nil {
			db.DPrintf(db.ERROR, "Err read boot script local: %v", err)
			return nil, err
		}
		wrt := NewWasmerRuntime(nil)
		return wrt.PrecompileModule(b)
	} else {
		return ReadCoSandboxRemote(sc, scriptName)
	}
}
