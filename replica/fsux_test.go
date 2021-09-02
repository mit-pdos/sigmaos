package replica

import (
	"testing"

	db "ulambda/debug"
	"ulambda/fslib"
	"ulambda/kernel"
	"ulambda/procinit"
)

func makeFsUxTstate(t *testing.T) *Tstate {
	ts := &Tstate{}

	bin := ".."
	s, err := kernel.Boot(bin)
	if err != nil {
		t.Fatalf("Boot %v\n", err)
	}
	ts.s = s

	procinit.SetProcLayers(map[string]bool{procinit.PROCBASE: true})

	replicaName := "fsux-replica"
	db.Name(replicaName + "-test")
	ts.FsLib = fslib.MakeFsLib(replicaName + "-test")
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
	ts.s.Shutdown(ts.FsLib)
}

// Test making & reading a few files.
func TestFsUxChainSimple(t *testing.T) {
	ts := makeFsUxTstate(t)
	ChainSimple(ts)
	ts.s.Shutdown(ts.FsLib)
}

// Test making & reading a few files in the presence of crashes in the middle of
// the chain
func TestFsUxChainCrashMiddle(t *testing.T) {
	ts := makeFsUxTstate(t)
	ChainCrashMiddle(ts)
	ts.s.Shutdown(ts.FsLib)
}

func TestFsUxChainCrashHead(t *testing.T) {
	ts := makeFsUxTstate(t)
	ChainCrashHead(ts)
	ts.s.Shutdown(ts.FsLib)
}

func TestFsUxChainCrashTail(t *testing.T) {
	ts := makeFsUxTstate(t)
	ChainCrashTail(ts)
	ts.s.Shutdown(ts.FsLib)
}

func TestFsUxConcurrentClientsSimple(t *testing.T) {
	ts := makeFsUxTstate(t)
	ConcurrentClientsSimple(ts)
	ts.s.Shutdown(ts.FsLib)
}

func TestFsUxConcurrentClientsCrashMiddle(t *testing.T) {
	ts := makeFsUxTstate(t)
	ConcurrentClientsCrashMiddle(ts)
	ts.s.Shutdown(ts.FsLib)
}

func TestFsUxConcurrentClientsCrashTail(t *testing.T) {
	ts := makeFsUxTstate(t)
	ConcurrentClientsCrashTail(ts)
	ts.s.Shutdown(ts.FsLib)
}

func TestFsUxConcurrentClientsCrashHead(t *testing.T) {
	ts := makeFsUxTstate(t)
	ConcurrentClientsCrashHead(ts)
	ts.s.Shutdown(ts.FsLib)
}

func TestFsUxConcurrentClientsCrashHeadNotIdempotent(t *testing.T) {
	ts := makeFsUxTstate(t)
	ConcurrentClientsCrashHeadNotIdempotent(ts)
	ts.s.Shutdown(ts.FsLib)
}
