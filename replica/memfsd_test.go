package replica

import (
	"testing"

	db "ulambda/debug"
	"ulambda/fslib"
)

func makeTstate(t *testing.T) *Tstate {
	ts := &Tstate{}

	bin := ".."
	s, err := fslib.Boot(bin)
	if err != nil {
		t.Fatalf("Boot %v\n", err)
	}
	ts.s = s

	replicaName := "memfs-replica"
	db.Name(replicaName + "-test")
	ts.FsLib = fslib.MakeFsLib(replicaName + "-test")
	ts.t = t
	ts.configPath9p = "name/" + replicaName + "-config.txt"
	ts.unionDirPath9p = "name/" + replicaName
	ts.replicaBin = "bin/" + replicaName
	return ts
}

func TestMemfsHelloWorld(t *testing.T) {
	ts := makeTstate(t)
	HelloWorld(ts)
	ts.s.Shutdown(ts.FsLib)
}

// Test making & reading a few files.
func TestMemfsChainSimple(t *testing.T) {
	ts := makeTstate(t)
	ChainSimple(ts)
	ts.s.Shutdown(ts.FsLib)
}

// Test making & reading a few files in the presence of crashes in the middle of
// the chain
func TestMemfsChainCrashMiddle(t *testing.T) {
	ts := makeTstate(t)
	ChainCrashMiddle(ts)
	ts.s.Shutdown(ts.FsLib)
}

func TestMemfsChainCrashHead(t *testing.T) {
	ts := makeTstate(t)
	ChainCrashHead(ts)
	ts.s.Shutdown(ts.FsLib)
}

func TestMemfsChainCrashTail(t *testing.T) {
	ts := makeTstate(t)
	ChainCrashTail(ts)
	ts.s.Shutdown(ts.FsLib)
}

func TestMemfsConcurrentClientsSimple(t *testing.T) {
	ts := makeTstate(t)
	ConcurrentClientsSimple(ts)
	ts.s.Shutdown(ts.FsLib)
}

func TestMemfsConcurrentClientsCrashMiddle(t *testing.T) {
	ts := makeTstate(t)
	ConcurrentClientsCrashMiddle(ts)
	ts.s.Shutdown(ts.FsLib)
}

func TestMemfsConcurrentClientsCrashTail(t *testing.T) {
	ts := makeTstate(t)
	ConcurrentClientsCrashTail(ts)
	ts.s.Shutdown(ts.FsLib)
}

func TestMemfsConcurrentClientsCrashHead(t *testing.T) {
	ts := makeTstate(t)
	ConcurrentClientsCrashHead(ts)
	ts.s.Shutdown(ts.FsLib)
}
