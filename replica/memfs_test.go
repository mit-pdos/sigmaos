package replica

import (
	"testing"

	db "ulambda/debug"
	"ulambda/fslib"
	"ulambda/procinit"
	"ulambda/realm"
)

func makeMemfsTstate(t *testing.T) *Tstate {
	ts := &Tstate{}

	bin := ".."
	e := realm.MakeTestEnv(bin)
	cfg, err := e.Boot()
	if err != nil {
		t.Fatalf("Boot %v\n", err)
	}
	ts.e = e
	ts.cfg = cfg

	procinit.SetProcLayers(map[string]bool{procinit.PROCBASE: true})

	replicaName := "memfs-chain-replica"
	db.Name(replicaName + "-test")
	ts.FsLib = fslib.MakeFsLibAddr(replicaName+"-test", cfg.NamedAddr)
	ts.t = t
	ts.configPath9p = "name/" + replicaName + "-config.txt"
	ts.unionDirPath9p = "name/" + replicaName
	ts.symlinkPath9p = "name/" + replicaName + "-HEAD"
	ts.replicaBin = "bin/user/" + replicaName
	return ts
}

func TestMemfsHelloWorld(t *testing.T) {
	ts := makeMemfsTstate(t)
	HelloWorld(ts)
	ts.e.Shutdown()
}

// Test making & reading a few files.
func TestMemfsChainSimple(t *testing.T) {
	ts := makeMemfsTstate(t)
	ChainSimple(ts)
	ts.e.Shutdown()
}

// Test making & reading a few files in the presence of crashes in the middle of
// the chain
func TestMemfsChainCrashMiddle(t *testing.T) {
	ts := makeMemfsTstate(t)
	ChainCrashMiddle(ts)
	ts.e.Shutdown()
}

func TestMemfsChainCrashHead(t *testing.T) {
	ts := makeMemfsTstate(t)
	ChainCrashHead(ts)
	ts.e.Shutdown()
}

func TestMemfsChainCrashTail(t *testing.T) {
	ts := makeMemfsTstate(t)
	ChainCrashTail(ts)
	ts.e.Shutdown()
}

func TestMemfsConcurrentClientsSimple(t *testing.T) {
	ts := makeMemfsTstate(t)
	ConcurrentClientsSimple(ts)
	ts.e.Shutdown()
}

func TestMemfsConcurrentClientsCrashMiddle(t *testing.T) {
	ts := makeMemfsTstate(t)
	ConcurrentClientsCrashMiddle(ts)
	ts.e.Shutdown()
}

func TestMemfsConcurrentClientsCrashTail(t *testing.T) {
	ts := makeMemfsTstate(t)
	ConcurrentClientsCrashTail(ts)
	ts.e.Shutdown()
}

func TestMemfsConcurrentClientsCrashHead(t *testing.T) {
	ts := makeMemfsTstate(t)
	ConcurrentClientsCrashHead(ts)
	ts.e.Shutdown()
}

func TestMemfsConcurrentClientsCrashHeadNotIdempotent(t *testing.T) {
	ts := makeMemfsTstate(t)
	ConcurrentClientsCrashHeadNotIdempotent(ts)
	ts.e.Shutdown()
}
