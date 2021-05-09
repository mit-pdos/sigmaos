package memfsd_replica

import (
	"log"
	"os"
	"os/exec"
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
	port string
	cmd  *exec.Cmd
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
	replica.cmd, err = run(bin, "bin/memfs-replica", []string{"placeholder-pid", replica.port, CONFIG_PATH_9P, UNION_DIR_PATH})
	assert.Nil(ts.t, err, "Failed to boot replica")
	time.Sleep(100 * time.Millisecond)
}

func killReplica(ts *Tstate, replica *Replica) {
	err := replica.cmd.Process.Kill()
	assert.Nil(ts.t, err, "Failed to kill replica")
}

func allocReplicas(n int) []*Replica {
	replicas := make([]*Replica, n)
	for i, _ := range replicas {
		replicas[i] = &Replica{strconv.Itoa(PORT_OFFSET + i), nil}
	}
	return replicas
}

func writeConfig(ts *Tstate, replicas []*Replica) {
	addrs := []string{}
	ip, err := fsclnt.LocalIP()
	assert.Nil(ts.t, err, "Failed to get local ip")
	for _, r := range replicas {
		addrs = append(addrs, ip+":"+r.port)
	}
	config := strings.Join(addrs, "\n")
	err = ts.MakeFile(CONFIG_PATH_9P, 0777, []byte(config))
	assert.Nil(ts.t, err, "Failed to make config file")
}

func setupUnionDir(ts *Tstate) {
	err := ts.Mkdir(UNION_DIR_PATH, 0777)
	assert.Nil(ts.t, err, "Failed to create union dir")
}

func TestHelloWorld(t *testing.T) {
	ts := makeTstate(t)

	N := 1

	replicas := allocReplicas(N)
	writeConfig(ts, replicas)
	setupUnionDir(ts)

	log.Printf("Hello world!")
	bootReplica(ts, replicas[0])
	killReplica(ts, replicas[0])

	ts.s.Shutdown(ts.FsLib)
}
