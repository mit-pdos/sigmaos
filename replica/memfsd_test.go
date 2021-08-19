package replica

import (
	"testing"

	db "ulambda/debug"
	"ulambda/fslib"
	"ulambda/kernel"
)

func makeMemfsTstate(t *testing.T) *Tstate {
	ts := &Tstate{}

	bin := ".."
	s, err := kernel.Boot(bin)
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
	ts.symlinkPath9p = "name/" + replicaName + "-HEAD"
	ts.replicaBin = "bin/kernel/" + replicaName
	return ts
}

func TestMemfsHelloWorld(t *testing.T) {
	ts := makeMemfsTstate(t)
	HelloWorld(ts)
	ts.s.Shutdown(ts.FsLib)
}

// Test making & reading a few files.
func TestMemfsChainSimple(t *testing.T) {
	ts := makeMemfsTstate(t)
	ChainSimple(ts)
	ts.s.Shutdown(ts.FsLib)
}

// Test making & reading a few files in the presence of crashes in the middle of
// the chain
func TestMemfsChainCrashMiddle(t *testing.T) {
	ts := makeMemfsTstate(t)
	ChainCrashMiddle(ts)
	ts.s.Shutdown(ts.FsLib)
}

func TestMemfsChainCrashHead(t *testing.T) {
	ts := makeMemfsTstate(t)
	ChainCrashHead(ts)
	ts.s.Shutdown(ts.FsLib)
}

func TestMemfsChainCrashTail(t *testing.T) {
	ts := makeMemfsTstate(t)
	ChainCrashTail(ts)
	ts.s.Shutdown(ts.FsLib)
}

func TestMemfsConcurrentClientsSimple(t *testing.T) {
	ts := makeMemfsTstate(t)
	ConcurrentClientsSimple(ts)
	ts.s.Shutdown(ts.FsLib)
}

func TestMemfsConcurrentClientsCrashMiddle(t *testing.T) {
	ts := makeMemfsTstate(t)
	ConcurrentClientsCrashMiddle(ts)
	ts.s.Shutdown(ts.FsLib)
}

func TestMemfsConcurrentClientsCrashTail(t *testing.T) {
	ts := makeMemfsTstate(t)
	ConcurrentClientsCrashTail(ts)
	ts.s.Shutdown(ts.FsLib)
}

func TestMemfsConcurrentClientsCrashHead(t *testing.T) {
	ts := makeMemfsTstate(t)
	ConcurrentClientsCrashHead(ts)
	ts.s.Shutdown(ts.FsLib)
}

func TestMemfsConcurrentClientsCrashHeadNotIdempotent(t *testing.T) {
	ts := makeMemfsTstate(t)
	ConcurrentClientsCrashHeadNotIdempotent(ts)
	ts.s.Shutdown(ts.FsLib)
}
