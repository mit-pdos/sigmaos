package replica

import (
	"testing"

	db "sigmaos/debug"
	"sigmaos/fslib"
	"sigmaos/realm"
)

func makeMemfsTstate(t *testing.T, name string, checkLogs bool) *Tstate {
	ts := &Tstate{}
	ts.checkLogs = checkLogs

	bin := ".."
	e := realm.MakeTestEnv(bin)
	cfg, err := e.Boot()
	if err != nil {
		t.Fatalf("Boot %v\n", err)
	}
	ts.e = e
	ts.cfg = cfg

	replicaName := name
	db.Name(replicaName + "-test")
	ts.FsLib = fslib.MakeFsLibAddr(replicaName+"-test", cfg.NamedAddrs)
	ts.t = t
	ts.configPath9p = "name/" + replicaName + "-config.txt"
	ts.unionDirPath9p = "name/" + replicaName
	ts.symlinkPath9p = "name/" + replicaName + "-HEAD"
	ts.replicaBin = "user/" + replicaName
	return ts
}

func TestMemfsHelloWorld(t *testing.T) {
	ts := makeMemfsTstate(t, "memfs-chain-replica", true)
	HelloWorld(ts)
	ts.e.Shutdown()
}

// Test making & reading a few files.
func TestMemfsChainSimple(t *testing.T) {
	ts := makeMemfsTstate(t, "memfs-chain-replica", true)
	ChainSimple(ts)
	ts.e.Shutdown()
}

// Test making & reading a few files in the presence of crashes in the middle of
// the chain
func TestMemfsChainCrashMiddle(t *testing.T) {
	ts := makeMemfsTstate(t, "memfs-chain-replica", true)
	ChainCrashMiddle(ts)
	ts.e.Shutdown()
}

func TestMemfsChainCrashHead(t *testing.T) {
	ts := makeMemfsTstate(t, "memfs-chain-replica", true)
	ChainCrashHead(ts)
	ts.e.Shutdown()
}

func TestMemfsChainCrashTail(t *testing.T) {
	ts := makeMemfsTstate(t, "memfs-chain-replica", true)
	ChainCrashTail(ts)
	ts.e.Shutdown()
}

func TestMemfsConcurrentClientsSimple(t *testing.T) {
	ts := makeMemfsTstate(t, "memfs-chain-replica", true)
	ConcurrentClientsSimple(ts)
	ts.e.Shutdown()
}

func TestMemfsConcurrentClientsCrashMiddle(t *testing.T) {
	ts := makeMemfsTstate(t, "memfs-chain-replica", true)
	ConcurrentClientsCrashMiddle(ts)
	ts.e.Shutdown()
}

func TestMemfsConcurrentClientsCrashTail(t *testing.T) {
	ts := makeMemfsTstate(t, "memfs-chain-replica", true)
	ConcurrentClientsCrashTail(ts)
	ts.e.Shutdown()
}

func TestMemfsConcurrentClientsCrashHead(t *testing.T) {
	ts := makeMemfsTstate(t, "memfs-chain-replica", true)
	ConcurrentClientsCrashHead(ts)
	ts.e.Shutdown()
}

func TestMemfsConcurrentClientsCrashHeadNotIdempotent(t *testing.T) {
	ts := makeMemfsTstate(t, "memfs-chain-replica", true)
	ConcurrentClientsCrashHeadNotIdempotent(ts)
	ts.e.Shutdown()
}
