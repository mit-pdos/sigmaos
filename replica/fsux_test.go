package replica

import (
	"testing"

	db "ulambda/debug"
	"ulambda/fslib"
	"ulambda/procinit"
	"ulambda/realm"
)

func makeFsUxTstate(t *testing.T) *Tstate {
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

	replicaName := "fsux-chain-replica"
	db.Name(replicaName + "-test")
	ts.FsLib = fslib.MakeFsLibAddr(replicaName+"-test", cfg.NamedAddr)
	ts.t = t
	ts.configPath9p = "name/" + replicaName + "-config.txt"
	ts.unionDirPath9p = "name/" + replicaName
	ts.symlinkPath9p = "name/" + replicaName + "-HEAD"
	ts.replicaBin = "bin/user/" + replicaName
	return ts
}

func TestFsUxHelloWorld(t *testing.T) {
	ts := makeFsUxTstate(t)
	HelloWorld(ts)
	ts.e.Shutdown()
}

// Test making & reading a few files.
func TestFsUxChainSimple(t *testing.T) {
	ts := makeFsUxTstate(t)
	ChainSimple(ts)
	ts.e.Shutdown()
}

// Test making & reading a few files in the presence of crashes in the middle of
// the chain
func TestFsUxChainCrashMiddle(t *testing.T) {
	ts := makeFsUxTstate(t)
	ChainCrashMiddle(ts)
	ts.e.Shutdown()
}

func TestFsUxChainCrashHead(t *testing.T) {
	ts := makeFsUxTstate(t)
	ChainCrashHead(ts)
	ts.e.Shutdown()
}

func TestFsUxChainCrashTail(t *testing.T) {
	ts := makeFsUxTstate(t)
	ChainCrashTail(ts)
	ts.e.Shutdown()
}

func TestFsUxConcurrentClientsSimple(t *testing.T) {
	ts := makeFsUxTstate(t)
	ConcurrentClientsSimple(ts)
	ts.e.Shutdown()
}

func TestFsUxConcurrentClientsCrashMiddle(t *testing.T) {
	ts := makeFsUxTstate(t)
	ConcurrentClientsCrashMiddle(ts)
	ts.e.Shutdown()
}

func TestFsUxConcurrentClientsCrashTail(t *testing.T) {
	ts := makeFsUxTstate(t)
	ConcurrentClientsCrashTail(ts)
	ts.e.Shutdown()
}

func TestFsUxConcurrentClientsCrashHead(t *testing.T) {
	ts := makeFsUxTstate(t)
	ConcurrentClientsCrashHead(ts)
	ts.e.Shutdown()
}

func TestFsUxConcurrentClientsCrashHeadNotIdempotent(t *testing.T) {
	ts := makeFsUxTstate(t)
	ConcurrentClientsCrashHeadNotIdempotent(ts)
	ts.e.Shutdown()
}
