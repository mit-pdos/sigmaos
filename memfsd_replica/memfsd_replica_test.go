package memfsd_replica

import (
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	db "ulambda/debug"
	"ulambda/fsclnt"
	"ulambda/fslib"
)

const (
	CONFIG_PATH_9P = "name/memfs-replica-config.txt"
	UNION_DIR_PATH = "name/memfsd-replicas"
	PORT_OFFSET    = 30001
)

type Replica struct {
	addr    string
	port    string
	crashed bool
	cmd     *exec.Cmd
}

type Tstate struct {
	*fslib.FsLib
	t *testing.T
	s *fslib.System
}

func makeTstate(t *testing.T) *Tstate {
	ts := &Tstate{}

	bin := ".."
	s, err := fslib.Boot(bin)
	if err != nil {
		t.Fatalf("Boot %v\n", err)
	}
	ts.s = s
	db.Name("memfsd_replica_test")

	ts.FsLib = fslib.MakeFsLib("memfsd_replica_test")
	ts.t = t
	return ts
}

func run(bin string, name string, args []string) (*exec.Cmd, error) {
	cmd := exec.Command(bin+"/"+name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ())
	return cmd, cmd.Start()
}

func bootReplica(ts *Tstate, replica *Replica) {
	bin := ".."
	var err error
	replica.cmd, err = run(bin, "bin/memfs-replica", []string{"placeholder-pid", replica.port, CONFIG_PATH_9P, UNION_DIR_PATH, "log-ops"})
	assert.Nil(ts.t, err, "Failed to boot replica")
	time.Sleep(100 * time.Millisecond)
}

func killReplica(ts *Tstate, replica *Replica) {
	err := replica.cmd.Process.Kill()
	assert.Nil(ts.t, err, "Failed to kill replica")
	time.Sleep(100 * time.Millisecond)
}

func allocReplicas(ts *Tstate, n int) []*Replica {
	replicas := make([]*Replica, n)
	ip, err := fsclnt.LocalIP()
	assert.Nil(ts.t, err, "Failed to get local ip")
	for i, _ := range replicas {
		portstr := strconv.Itoa(PORT_OFFSET + i)
		replicas[i] = &Replica{ip + ":" + portstr, portstr, false, nil}
	}
	return replicas
}

func writeConfig(ts *Tstate, replicas []*Replica) {
	addrs := []string{}
	for _, r := range replicas {
		addrs = append(addrs, r.addr)
	}
	config := strings.Join(addrs, "\n")
	err := ts.MakeFile(CONFIG_PATH_9P, 0777, []byte(config))
	assert.Nil(ts.t, err, "Failed to make config file")
}

func setupUnionDir(ts *Tstate) {
	err := ts.Mkdir(UNION_DIR_PATH, 0777)
	assert.Nil(ts.t, err, "Failed to create union dir")
}

func compareReplicaLogs(ts *Tstate, replicas []*Replica) {
	if len(replicas) < 2 {
		return
	}
	logs := [][]byte{}
	for _, r := range replicas {
		// If this replica was not killed...
		if !r.crashed {
			b, err := ts.ReadFile(path.Join("name", r.addr+"-log.txt"))
			assert.Nil(ts.t, err, "Failed to read log file for replica: %v", r.addr)
			logs = append(logs, b)
		}
	}

	for i, l := range logs {
		assert.Greater(ts.t, len(l), 0, "Zero length log")
		if i > 0 {
			assert.ElementsMatch(ts.t, logs[i-1], l, "Logs do not match: %v, %v", i-1, i)
		}
	}
}

func TestHelloWorld(t *testing.T) {
	ts := makeTstate(t)

	N := 1

	replicas := allocReplicas(ts, N)
	writeConfig(ts, replicas)
	setupUnionDir(ts)

	// Start up
	for _, r := range replicas {
		bootReplica(ts, r)
	}

	time.Sleep(200 * time.Millisecond)

	// Shut down
	for _, r := range replicas {
		killReplica(ts, r)
	}

	ts.s.Shutdown(ts.FsLib)
}

// Test a few ops on the chain.
func TestChainSimple(t *testing.T) {
	ts := makeTstate(t)

	N := 5
	n_files := 100

	replicas := allocReplicas(ts, N)
	writeConfig(ts, replicas)
	setupUnionDir(ts)

	// Start up
	for _, r := range replicas {
		bootReplica(ts, r)
	}

	time.Sleep(1000 * time.Millisecond)

	// Write some files to the head
	for i := 0; i < n_files; i++ {
		i_str := strconv.Itoa(i)
		err := ts.MakeFile(path.Join(UNION_DIR_PATH, replicas[0].addr, i_str), 0777, []byte(i_str))
		assert.Nil(ts.t, err, "Failed to MakeFile in head")
	}

	// Read some files from the tail
	for i := 0; i < n_files; i++ {
		i_str := strconv.Itoa(i)
		b, err := ts.ReadFile(path.Join(UNION_DIR_PATH, replicas[0].addr, i_str))
		assert.Nil(ts.t, err, "Failed to ReadFile from tail")
		assert.Equal(ts.t, string(b), i_str, "File contents not equal")
	}

	// Wait a bit to allow replica logs to stabilize
	time.Sleep(1000 * time.Millisecond)

	compareReplicaLogs(ts, replicas)

	// Shut down
	for _, r := range replicas {
		killReplica(ts, r)
	}

	ts.s.Shutdown(ts.FsLib)
}
