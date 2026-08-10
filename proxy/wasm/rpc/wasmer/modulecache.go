package wasmer

// A process-wide cache of compiled WASM modules, keyed by the pathname they
// were compiled from.
//
// Compiling a co-sandbox is the expensive part of starting one, and every proc
// on a node that uses the same co-sandbox compiles the same bytes. procd fetches
// the binary once per node (ProcSrv.downloadCoSandbox); this makes the
// compilation once per node too, so the Nth proc pays only for instantiation.
//
// The cache is never invalidated. Keys are pathnames in procd's bin cache, whose
// contents are written once by chunksrv and not rewritten, so a live entry
// cannot go stale — see wasmer.CoSandboxProg for the one case (a cluster
// outliving a rebuild) where the file itself could be a previous build's.

import (
	"os"
	"sync"
	"time"

	wasmer "github.com/wasmerio/wasmer-go/wasmer"

	db "sigmaos/debug"
	sp "sigmaos/sigmap"
	"sigmaos/util/perf"
)

// cachedModule is a compiled module and the store it belongs to. wasmer ties a
// module, its store, and the imports of any instance of it together, so the two
// are cached and handed out as a unit.
type cachedModule struct {
	// Serializes use of the shared store. See runModule.
	mu     sync.Mutex
	store  *wasmer.Store
	module *wasmer.Module
}

func newCachedModule(store *wasmer.Store, module *wasmer.Module) *cachedModule {
	return &cachedModule{store: store, module: module}
}

var (
	moduleCacheMu sync.Mutex
	moduleCache   = make(map[string]*cachedModule)
)

// loadModule returns the compiled module for the co-sandbox binary at pn,
// compiling it on first use.
//
// The lock is held across the compile so that N procs racing for the same
// co-sandbox compile it once rather than N times — the whole point of the
// cache, and worth blocking the losers, who would otherwise each do the work
// this saves. Different co-sandboxes contend on it too, which is acceptable:
// there are a handful per realm and each is compiled once.
func loadModule(pid sp.Tpid, pn string) (*cachedModule, error) {
	moduleCacheMu.Lock()
	defer moduleCacheMu.Unlock()

	if cm, ok := moduleCache[pn]; ok {
		db.DPrintf(db.WASMRT, "[%v] Cached WASM module %v", pid, pn)
		return cm, nil
	}
	start := time.Now()
	b, err := os.ReadFile(pn)
	if err != nil {
		db.DPrintf(db.ERROR, "[%v] Err read co-sandbox %v: %v", pid, pn, err)
		db.DPrintf(db.WASMRT_ERR, "[%v] Err read co-sandbox %v: %v", pid, pn, err)
		return nil, err
	}
	cfg := wasmer.NewConfig().UseCraneliftCompiler()
	store := wasmer.NewStore(wasmer.NewEngineWithConfig(cfg))
	module, err := wasmer.NewModule(store, b)
	if err != nil {
		db.DPrintf(db.ERROR, "[%v] Err compile co-sandbox %v: %v", pid, pn, err)
		db.DPrintf(db.WASMRT_ERR, "[%v] Err compile co-sandbox %v: %v", pid, pn, err)
		return nil, err
	}
	perf.LogSpawnLatency("WASM module compilation (%v, %vB)", pid, perf.TIME_NOT_SET, start, pn, len(b))
	cm := newCachedModule(store, module)
	moduleCache[pn] = cm
	return cm, nil
}
