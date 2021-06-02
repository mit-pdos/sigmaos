package replica

import (
	"testing"

	db "ulambda/debug"
	"ulambda/fslib"
)

func makeNpUxTstate(t *testing.T) *Tstate {
	ts := &Tstate{}

	bin := ".."
	s, err := fslib.Boot(bin)
	if err != nil {
		t.Fatalf("Boot %v\n", err)
	}
	ts.s = s

	replicaName := "npux-replica"
	db.Name(replicaName + "-test")
	ts.FsLib = fslib.MakeFsLib(replicaName + "-test")
	ts.t = t
	ts.configPath9p = "name/" + replicaName + "-config.txt"
	ts.unionDirPath9p = "name/" + replicaName
	ts.replicaBin = "bin/" + replicaName
	return ts
}

func TestNpUxHelloWorld(t *testing.T) {
	ts := makeNpUxTstate(t)
	HelloWorld(ts)
	ts.s.Shutdown(ts.FsLib)
}

// Test making & reading a few files.
func TestNpUxChainSimple(t *testing.T) {
	ts := makeNpUxTstate(t)
	ChainSimple(ts)
	ts.s.Shutdown(ts.FsLib)
}

// Test making & reading a few files in the presence of crashes in the middle of
// the chain
func TestNpUxChainCrashMiddle(t *testing.T) {
	ts := makeNpUxTstate(t)
	ChainCrashMiddle(ts)
	ts.s.Shutdown(ts.FsLib)
}

func TestNpUxChainCrashHead(t *testing.T) {
	ts := makeNpUxTstate(t)
	ChainCrashHead(ts)
	ts.s.Shutdown(ts.FsLib)
}

func TestNpUxChainCrashTail(t *testing.T) {
	ts := makeNpUxTstate(t)
	ChainCrashTail(ts)
	ts.s.Shutdown(ts.FsLib)
}

func TestNpUxConcurrentClientsSimple(t *testing.T) {
	ts := makeNpUxTstate(t)
	ConcurrentClientsSimple(ts)
	ts.s.Shutdown(ts.FsLib)
}

func TestNpUxConcurrentClientsCrashMiddle(t *testing.T) {
	ts := makeNpUxTstate(t)
	ConcurrentClientsCrashMiddle(ts)
	ts.s.Shutdown(ts.FsLib)
}

func TestNpUxConcurrentClientsCrashTail(t *testing.T) {
	ts := makeNpUxTstate(t)
	ConcurrentClientsCrashTail(ts)
	ts.s.Shutdown(ts.FsLib)
}

func TestNpUxConcurrentClientsCrashHead(t *testing.T) {
	ts := makeNpUxTstate(t)
	ConcurrentClientsCrashHead(ts)
	ts.s.Shutdown(ts.FsLib)
}
