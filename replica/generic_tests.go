package replica

import (
	"log"
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	db "ulambda/debug"
	"ulambda/fidclnt"
	"ulambda/fslib"
	np "ulambda/ninep"
	"ulambda/realm"
)

const (
	PORT_OFFSET      = 30001
	MAX_OPEN_RETRIES = 1000
)

type Replica struct {
	addr    string
	port    string
	crashed bool
	cmd     *exec.Cmd
}

type Tstate struct {
	replicaBin     string
	configPath9p   string
	unionDirPath9p string
	symlinkPath9p  string
	checkLogs      bool
	t              *testing.T
	e              *realm.TestEnv
	cfg            *realm.RealmConfig
	*fslib.FsLib
}

func run(bin string, name string, args []string, namedAddr []string) (*exec.Cmd, error) {
	cmd := exec.Command(bin+"/"+name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ(), "NAMED="+strings.Join(namedAddr, ","))
	return cmd, cmd.Start()
}

func bootReplica(ts *Tstate, replica *Replica) {
	bin := ".."
	var err error
	replica.cmd, err = run(bin,
		ts.replicaBin,
		[]string{"placeholder-pid",
			replica.port,
			ts.configPath9p,
			ts.unionDirPath9p,
			ts.symlinkPath9p,
			"log-ops"},
		ts.cfg.NamedAddr)
	assert.Nil(ts.t, err, "Failed to boot replica")
	time.Sleep(100 * time.Millisecond)
}

func crashReplica(ts *Tstate, replica *Replica) {
	killReplica(ts, replica)
	replica.crashed = true
}

func killReplica(ts *Tstate, replica *Replica) {
	err := replica.cmd.Process.Kill()
	assert.Nil(ts.t, err, "Failed to kill replica")
	time.Sleep(100 * time.Millisecond)
}

func allocReplicas(ts *Tstate, n int) []*Replica {
	replicas := make([]*Replica, n)
	ip, err := fidclnt.LocalIP()
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
	_, err := ts.PutFile(ts.configPath9p, 0777, np.OWRITE, []byte(config))
	assert.Nil(ts.t, err, "Failed to make config file: %v", ts.configPath9p)
}

func setupUnionDir(ts *Tstate) {
	err := ts.MkDir(ts.unionDirPath9p, 0777)
	assert.Nil(ts.t, err, "Failed to create union dir")
}

func compareReplicaLogs(ts *Tstate, replicas []*Replica) {
	if len(replicas) < 2 {
		return
	}
	logs := [][]byte{}
	idxMap := map[int]string{}
	for _, r := range replicas {
		// If this replica was not killed...
		if !r.crashed {
			b, err := ts.GetFile(path.Join("name", r.addr+"-log.txt"))
			assert.Nil(ts.t, err, "Failed to read log file for replica: %v", r.addr)
			idxMap[len(logs)] = r.addr
			logs = append(logs, b)
		}
	}

	for i, l := range logs {
		assert.Greater(ts.t, len(l), 0, "Zero length log for log idx %v", i)
		if i > 0 {
			assert.ElementsMatch(ts.t, logs[i-1], l, "Logs do not match: %v, %v", idxMap[i-1], idxMap[i])
		}
	}
}

// Check that the contents of all files are present & correct on all replicas
func checkFiles(ts *Tstate, replicas []*Replica, n_files int) {
	for _, r := range replicas {
		// TODO: check tail too
		if !r.crashed && !isTail(ts, r, replicas) {
			for i := 0; i < n_files; i++ {
				i_str := strconv.Itoa(i)
				b, err := ts.GetFile(path.Join(ts.unionDirPath9p, r.addr, i_str))
				assert.Nil(ts.t, err, "Failed to ReadFile from replica: %v, %v", r.addr, path.Join(ts.unionDirPath9p, r.addr, i_str))
				assert.Equal(ts.t, string(b), i_str, "File contents not equal")
			}
		}
	}
}

// Check if this replica is currently the head
func isHead(ts *Tstate, replica *Replica, replicas []*Replica) bool {
	return strings.Contains(headPath(ts, replicas), replica.addr)
}

// Check if this replica is currently the tail
func isTail(ts *Tstate, replica *Replica, replicas []*Replica) bool {
	return strings.Contains(tailPath(ts, replicas), replica.addr)
}

// Calculate the ZK path to the head: the first un-crashed server in the chain
func headPath(ts *Tstate, replicas []*Replica) string {
	for _, r := range replicas {
		if !r.crashed {
			return path.Join(ts.unionDirPath9p, r.addr)
		}
	}
	return ""
}

// Calculate the ZK path to the tail: the last un-crashed server in the chain
func tailPath(ts *Tstate, replicas []*Replica) string {
	for i := len(replicas) - 1; i >= 0; i-- {
		if !replicas[i].crashed {
			return path.Join(ts.unionDirPath9p, replicas[i].addr)
		}
	}
	return ""
}

func HelloWorld(ts *Tstate) {
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
}

// Test making & reading a few files.
func ChainSimple(ts *Tstate) {
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
	log.Printf("Writing some files...")
	for i := 0; i < n_files; i++ {
		i_str := strconv.Itoa(i)
		f_path := path.Join(ts.symlinkPath9p, i_str) + "/"
		_, err := ts.PutFile(f_path, 0777, np.OWRITE, []byte(i_str))
		assert.Nil(ts.t, err, "Failed to MakeFile %v in head", f_path)
	}

	log.Printf("Done writing files...")

	// Read some files from the head
	log.Printf("Reading files...")
	for i := 0; i < n_files; i++ {
		i_str := strconv.Itoa(i)
		b, err := ts.GetFile(path.Join(ts.symlinkPath9p, i_str))
		assert.Nil(ts.t, err, "Failed to ReadFile from tail")
		assert.Equal(ts.t, string(b), i_str, "File contents not equal")
	}
	log.Printf("Done reading files...")

	// Wait a bit to allow replica logs to stabilize
	time.Sleep(1000 * time.Millisecond)

	if ts.checkLogs {
		log.Printf("Comparing replica logs...")
		compareReplicaLogs(ts, replicas)
		log.Printf("Done comparing replica logs...")
	}

	log.Printf("Checking file contents on each replica...")
	checkFiles(ts, replicas, n_files)
	log.Printf("Done checking file contents on each replica...")

	// Shut down
	for _, r := range replicas {
		killReplica(ts, r)
	}
}

// Test making & reading a few files in the presence of crashes in the middle of
// the chain
func ChainCrashMiddle(ts *Tstate) {
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
	log.Printf("Writing some files...")
	for i := 0; i < n_files; i++ {
		i_str := strconv.Itoa(i)
		_, err := ts.PutFile(path.Join(ts.symlinkPath9p, i_str), 0777, np.OWRITE, []byte(i_str))
		assert.Nil(ts.t, err, "Failed to MakeFile in head")
	}
	log.Printf("Done writing files...")

	// Crash a couple of replicas in the middle of the chain
	log.Printf("Crashing replicas %v and %v...", replicas[1].addr, replicas[2].addr)
	crashReplica(ts, replicas[1])
	crashReplica(ts, replicas[2])
	log.Printf("Done crashing replicas %v and %v...", replicas[1].addr, replicas[2].addr)

	time.Sleep(200 * time.Millisecond)

	// Read some files from the head
	log.Printf("Reading files...")
	for i := 0; i < n_files; i++ {
		i_str := strconv.Itoa(i)
		b, err := ts.GetFile(path.Join(ts.symlinkPath9p, i_str))
		assert.Nil(ts.t, err, "Failed to ReadFile from tail")
		assert.Equal(ts.t, string(b), i_str, "File contents not equal")
	}
	log.Printf("Done reading files...")

	// Wait a bit to allow replica logs to stabilize
	time.Sleep(1000 * time.Millisecond)

	if ts.checkLogs {
		log.Printf("Comparing replica logs...")
		compareReplicaLogs(ts, replicas)
		log.Printf("Done comparing replica logs...")
	}

	log.Printf("Checking file contents on each replica...")
	checkFiles(ts, replicas, n_files)
	log.Printf("Done checking file contents on each replica...")

	// Shut down
	for _, r := range replicas {
		killReplica(ts, r)
	}
}

func ChainCrashHead(ts *Tstate) {
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
	log.Printf("Writing some files...")
	for i := 0; i < n_files; i++ {
		i_str := strconv.Itoa(i)
		_, err := ts.PutFile(path.Join(ts.symlinkPath9p, i_str), 0777, np.OWRITE, []byte(i_str))
		assert.Nil(ts.t, err, "Failed to MakeFile in head")
	}
	log.Printf("Done writing files...")

	time.Sleep(500 * time.Millisecond)

	// Crash a couple of replicas in the middle of the chain
	log.Printf("Crashing head replica %v...", replicas[0].addr)
	crashReplica(ts, replicas[0])
	log.Printf("Done crashing head replica %v...", replicas[0].addr)

	time.Sleep(1000 * time.Millisecond)

	// Read some files from the head
	log.Printf("Reading files...")
	for i := 0; i < n_files; i++ {
		i_str := strconv.Itoa(i)
		b, err := ts.GetFile(path.Join(ts.symlinkPath9p, i_str))
		assert.Nil(ts.t, err, "Failed to ReadFile from tail")
		assert.Equal(ts.t, string(b), i_str, "File contents not equal")
	}
	log.Printf("Done reading files...")

	// Wait a bit to allow replica logs to stabilize
	time.Sleep(1000 * time.Millisecond)

	if ts.checkLogs {
		log.Printf("Comparing replica logs...")
		compareReplicaLogs(ts, replicas)
		log.Printf("Done comparing replica logs...")
	}

	log.Printf("Checking file contents on each replica...")
	checkFiles(ts, replicas, n_files)
	log.Printf("Done checking file contents on each replica...")

	// Shut down
	for _, r := range replicas {
		killReplica(ts, r)
	}
}

func ChainCrashTail(ts *Tstate) {
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
	log.Printf("Writing some files...")
	for i := 0; i < n_files; i++ {
		i_str := strconv.Itoa(i)
		_, err := ts.PutFile(path.Join(ts.symlinkPath9p, i_str), 0777, np.OWRITE, []byte(i_str))
		assert.Nil(ts.t, err, "Failed to MakeFile in head")
	}
	log.Printf("Done writing files...")

	// Crash a couple of replicas in the middle of the chain
	log.Printf("Crashing tail replica %v...", replicas[N-1].addr)
	crashReplica(ts, replicas[N-1])
	log.Printf("Done crashing tail replica %v...", replicas[N-1].addr)

	time.Sleep(200 * time.Millisecond)

	// Read some files from the head
	log.Printf("Reading files...")
	for i := 0; i < n_files; i++ {
		i_str := strconv.Itoa(i)
		b, err := ts.GetFile(path.Join(ts.symlinkPath9p, i_str))
		assert.Nil(ts.t, err, "Failed to ReadFile from tail")
		assert.Equal(ts.t, string(b), i_str, "File contents not equal")
	}
	log.Printf("Done reading files...")

	// Wait a bit to allow replica logs to stabilize
	time.Sleep(1000 * time.Millisecond)

	if ts.checkLogs {
		log.Printf("Comparing replica logs...")
		compareReplicaLogs(ts, replicas)
		log.Printf("Done comparing replica logs...")
	}

	log.Printf("Checking file contents on each replica...")
	checkFiles(ts, replicas, n_files)
	log.Printf("Done checking file contents on each replica...")

	// Shut down
	for _, r := range replicas {
		killReplica(ts, r)
	}
}

func basicClient(ts *Tstate, replicas []*Replica, id int, n_files int, start *sync.WaitGroup, end *sync.WaitGroup) {
	defer end.Done()

	fsl := fslib.MakeFsLibAddr("client-"+strconv.Itoa(id), ts.cfg.NamedAddr)
	start.Done()
	start.Wait()
	for i := 0; i < n_files; i++ {
		i_str := strconv.Itoa(id*n_files + i)
		_, err := fsl.PutFile(path.Join(ts.symlinkPath9p, i_str), 0777, np.OWRITE, []byte(i_str))
		assert.Nil(ts.t, err, "Failed to MakeFile in head")
	}
	for i := 0; i < n_files; i++ {
		i_str := strconv.Itoa(id*n_files + i)
		b, err := fsl.GetFile(path.Join(ts.symlinkPath9p, i_str))
		assert.Nil(ts.t, err, "Failed to ReadFile from tail")
		assert.Equal(ts.t, string(b), i_str, "File contents not equal")
	}
}

func ConcurrentClientsSimple(ts *Tstate) {
	N := 5
	n_clients := 10
	n_files_per_cli := 10

	replicas := allocReplicas(ts, N)
	writeConfig(ts, replicas)
	setupUnionDir(ts)

	// Start up
	for _, r := range replicas {
		bootReplica(ts, r)
	}

	time.Sleep(1000 * time.Millisecond)

	var start sync.WaitGroup
	var end sync.WaitGroup

	start.Add(n_clients)
	end.Add(n_clients)

	// Write some files to the head
	log.Printf("Starting clients...")
	for id := 0; id < n_clients; id++ {
		go basicClient(ts, replicas, id, n_files_per_cli, &start, &end)
	}

	log.Printf("Waiting for clients to terminate...")
	end.Wait()
	log.Printf("Done waiting for clients to terminate...")

	// Wait a bit to allow replica logs to stabilize
	time.Sleep(1000 * time.Millisecond)

	if ts.checkLogs {
		log.Printf("Comparing replica logs...")
		compareReplicaLogs(ts, replicas)
		log.Printf("Done comparing replica logs...")
	}

	log.Printf("Checking file contents on each replica...")
	checkFiles(ts, replicas, n_files_per_cli*n_clients)
	log.Printf("Done checking file contents on each replica...")

	// Shut down
	for _, r := range replicas {
		killReplica(ts, r)
	}
}

func ConcurrentClientsCrashMiddle(ts *Tstate) {
	N := 5
	n_clients := 10
	n_files_per_cli := 10

	replicas := allocReplicas(ts, N)
	writeConfig(ts, replicas)
	setupUnionDir(ts)

	// Start up
	for _, r := range replicas {
		bootReplica(ts, r)
	}

	time.Sleep(1000 * time.Millisecond)

	var start sync.WaitGroup
	var end sync.WaitGroup

	start.Add(n_clients)
	end.Add(n_clients)

	// Write some files to the head
	log.Printf("Starting clients...")
	for id := 0; id < n_clients; id++ {
		go basicClient(ts, replicas, id, n_files_per_cli, &start, &end)
	}

	// Crash a couple of replicas in the middle of the chain
	log.Printf("Crashing replicas %v and %v...", replicas[1].addr, replicas[2].addr)
	crashReplica(ts, replicas[1])
	crashReplica(ts, replicas[2])
	log.Printf("Done crashing replicas %v and %v...", replicas[1].addr, replicas[2].addr)

	log.Printf("Waiting for clients to terminate...")
	end.Wait()
	log.Printf("Done waiting for clients to terminate...")

	// Wait a bit to allow replica logs to stabilize
	time.Sleep(1000 * time.Millisecond)

	if ts.checkLogs {
		log.Printf("Comparing replica logs...")
		compareReplicaLogs(ts, replicas)
		log.Printf("Done comparing replica logs...")
	}

	log.Printf("Checking file contents on each replica...")
	checkFiles(ts, replicas, n_files_per_cli*n_clients)
	log.Printf("Done checking file contents on each replica...")

	// Shut down
	for _, r := range replicas {
		killReplica(ts, r)
	}
}

func ConcurrentClientsCrashTail(ts *Tstate) {
	N := 5
	n_clients := 10
	n_files_per_cli := 10

	replicas := allocReplicas(ts, N)
	writeConfig(ts, replicas)
	setupUnionDir(ts)

	// Start up
	for _, r := range replicas {
		bootReplica(ts, r)
	}

	time.Sleep(1000 * time.Millisecond)

	var start sync.WaitGroup
	var end sync.WaitGroup

	start.Add(n_clients)
	end.Add(n_clients)

	// Write some files to the head
	log.Printf("Starting clients...")
	for id := 0; id < n_clients; id++ {
		go basicClient(ts, replicas, id, n_files_per_cli, &start, &end)
	}

	// Crash a couple of replicas in the middle of the chain
	log.Printf("Crashing tail replica %v...", replicas[N-1].addr)
	crashReplica(ts, replicas[N-1])
	log.Printf("Done crashing tail replica %v...", replicas[N-1].addr)

	log.Printf("Waiting for clients to terminate...")
	end.Wait()
	log.Printf("Done waiting for clients to terminate...")

	// Wait a bit to allow replica logs to stabilize
	time.Sleep(1000 * time.Millisecond)

	if ts.checkLogs {
		log.Printf("Comparing replica logs...")
		compareReplicaLogs(ts, replicas)
		log.Printf("Done comparing replica logs...")
	}

	log.Printf("Checking file contents on each replica...")
	checkFiles(ts, replicas, n_files_per_cli*n_clients)
	log.Printf("Done checking file contents on each replica...")

	// Shut down
	for _, r := range replicas {
		killReplica(ts, r)
	}
}

func pausedClient(ts *Tstate, replicas []*Replica, id int, n_files int, start *sync.WaitGroup, end *sync.WaitGroup, writes *sync.WaitGroup, reads *sync.WaitGroup) {
	defer end.Done()

	fsl := fslib.MakeFsLibAddr("client-"+strconv.Itoa(id), ts.cfg.NamedAddr)
	start.Done()
	start.Wait()
	for i := 0; i < n_files; i++ {
		i_str := strconv.Itoa(id*n_files + i)
		_, err := fsl.PutFile(path.Join(ts.symlinkPath9p, i_str), 0777, np.OWRITE, []byte(i_str))
		assert.Nil(ts.t, err, "Failed to MakeFile in head")
	}
	writes.Done()
	reads.Wait()
	for i := 0; i < n_files; i++ {
		i_str := strconv.Itoa(id*n_files + i)
		fpath := path.Join(ts.symlinkPath9p, i_str)
		b, err := fsl.GetFile(fpath)
		assert.Nil(ts.t, err, "Failed to ReadFile path: %v", fpath)
		assert.Equal(ts.t, i_str, string(b), "File contents not equal")
	}
}

func ConcurrentClientsCrashHead(ts *Tstate) {
	N := 5
	n_clients := 10
	n_files_per_cli := 10

	replicas := allocReplicas(ts, N)
	writeConfig(ts, replicas)
	setupUnionDir(ts)

	// Start up
	for _, r := range replicas {
		bootReplica(ts, r)
	}

	time.Sleep(1000 * time.Millisecond)

	var start sync.WaitGroup
	var end sync.WaitGroup
	var writes sync.WaitGroup
	var reads sync.WaitGroup

	start.Add(n_clients)
	end.Add(n_clients)
	writes.Add(n_clients)
	reads.Add(1)

	// Write some files to the head
	log.Printf("Starting clients...")
	for id := 0; id < n_clients; id++ {
		go pausedClient(ts, replicas, id, n_files_per_cli, &start, &end, &writes, &reads)
	}

	log.Printf("Waiting for clients to finish writes...")
	writes.Wait()
	log.Printf("Done waiting for clients to finish writes...")

	// Crash a couple of replicas in the middle of the chain
	log.Printf("Crashing head replica %v...", replicas[0].addr)
	crashReplica(ts, replicas[0])
	log.Printf("Done crashing head replica %v...", replicas[0].addr)

	log.Printf("Releasing clients to commence reads...")
	reads.Done()

	log.Printf("Waiting for clients to terminate...")
	end.Wait()
	log.Printf("Done waiting for clients to terminate...")

	// Wait a bit to allow replica logs to stabilize
	time.Sleep(1000 * time.Millisecond)

	if ts.checkLogs {
		log.Printf("Comparing replica logs...")
		compareReplicaLogs(ts, replicas)
		log.Printf("Done comparing replica logs...")
	}

	log.Printf("Checking file contents on each replica...")
	checkFiles(ts, replicas, n_files_per_cli*n_clients)
	log.Printf("Done checking file contents on each replica...")

	// Shut down
	for _, r := range replicas {
		killReplica(ts, r)
	}
}

type fn func() (string, error)
type assertion func(res string, err error) bool

// Return true if the cause of the error was a missing replica symlink
func symlinkWasMissing(ts *Tstate, err error) bool {
	return err != nil && err.Error() == "file not found "+path.Base(ts.symlinkPath9p)
}

func retryOp(ts *Tstate, f fn, a assertion) bool {
	for n_retries := 0; n_retries < MAX_OPEN_RETRIES; n_retries++ {
		db.DLPrintf("TEST", "Pre-op: %v", n_retries)
		res, err := f()
		db.DLPrintf("TEST", "Retry: %v, %v", n_retries, err)
		if symlinkWasMissing(ts, err) {
			continue
		}
		return a(res, err)
	}
	// Flag the error
	assert.True(ts.t, false, "Op failed to execute in %v retries", MAX_OPEN_RETRIES)
	return false
}

func renameClient(ts *Tstate, replicas []*Replica, id int, n_renames int, start *sync.WaitGroup, end *sync.WaitGroup) {
	defer end.Done()

	id_str := strconv.Itoa(id)

	fsl := fslib.MakeFsLibAddr("client-"+strconv.Itoa(id), ts.cfg.NamedAddr)
	start.Done()
	start.Wait()
	for i := 0; i < n_renames; i++ {
		old := id_str + "_" + strconv.Itoa(i)
		new := id_str + "_" + strconv.Itoa(i+1)
		oldpath := path.Join(ts.symlinkPath9p, old)
		newpath := path.Join(ts.symlinkPath9p, new)
		db.DLPrintf("TEST", "%v Start rename %v -> %v", id, old, new)
		retryOp(ts,
			func() (string, error) {
				db.DLPrintf("TEST", oldpath, newpath)
				return "", fsl.Rename(oldpath, newpath)
			},
			func(res string, err error) bool {
				return assert.Nil(ts.t, err, "Failed to rename: %v -> %v, %v", oldpath, newpath, err)
			})
		db.DLPrintf("TEST", "%v Finish rename %v -> %v", id, oldpath, newpath)
	}

	// Check the file contents remain unchanged
	fname := id_str + "_" + strconv.Itoa(n_renames)
	fpath := path.Join(ts.symlinkPath9p, fname)
	db.DLPrintf("TEST", "%v Start check file contents %v", id, fname)
	success := retryOp(ts,
		func() (string, error) {
			b, err := fsl.GetFile(fpath)
			return string(b), err
		},
		func(res string, err error) bool {
			return assert.Nil(ts.t, err, "Rename client failed to ReadFile: %v, %v", fpath, err) && assert.Equal(ts.t, id_str, res, "Renamed file contents not equal")
		})
	db.DLPrintf("TEST", "%v Finish check file contents %v", id, fname)

	if success {
		db.DLPrintf("TEST", "%v Start final rename %v -> %v", id, fname, path.Join(ts.symlinkPath9p, id_str))
		retryOp(ts,
			func() (string, error) {
				// Rename the file to its final name for our consistency checks
				return "", ts.Rename(fpath, path.Join(ts.symlinkPath9p, id_str))
			},
			func(res string, err error) bool {
				return assert.Nil(ts.t, err, "Final rename: %v, %v", id_str, err)
			})
		db.DLPrintf("TEST", "%v Finish final rename %v -> %v", id, fname, path.Join(ts.symlinkPath9p, id_str))
	}
}

func setupRenameableFiles(ts *Tstate, n_clients int) {
	for i := 0; i < n_clients; i++ {
		i_str := strconv.Itoa(i)
		fname := i_str + "_0"
		_, err := ts.PutFile(path.Join(ts.symlinkPath9p, fname), 0777, np.OWRITE, []byte(i_str))
		assert.Nil(ts.t, err, "Failed to MakeFile Renameable File: %v", err)
	}
}

func ConcurrentClientsCrashHeadNotIdempotent(ts *Tstate) {
	N := 5
	n_clients := 10
	n_renames := 10

	replicas := allocReplicas(ts, N)
	writeConfig(ts, replicas)
	setupUnionDir(ts)

	// Start up
	for _, r := range replicas {
		bootReplica(ts, r)
	}

	time.Sleep(1000 * time.Millisecond)

	setupRenameableFiles(ts, n_clients)

	var start sync.WaitGroup
	var end sync.WaitGroup

	start.Add(n_clients)
	end.Add(n_clients)

	// Write some files to the head
	log.Printf("Starting clients...")
	for id := 0; id < n_clients; id++ {
		go renameClient(ts, replicas, id, n_renames, &start, &end)
	}

	time.Sleep(100 * time.Millisecond)

	// Crash a couple of replicas in the middle of the chain
	log.Printf("Crashing head replica %v...", replicas[0].addr)
	crashReplica(ts, replicas[0])
	log.Printf("Done crashing head replica %v...", replicas[0].addr)

	log.Printf("Waiting for clients to terminate...")
	end.Wait()
	log.Printf("Done waiting for clients to terminate...")

	// Wait a bit to allow replica logs to stabilize
	time.Sleep(1000 * time.Millisecond)

	if ts.checkLogs {
		log.Printf("Comparing replica logs...")
		compareReplicaLogs(ts, replicas)
		log.Printf("Done comparing replica logs...")
	}

	log.Printf("Checking file contents on each replica...")
	checkFiles(ts, replicas, n_clients)
	log.Printf("Done checking file contents on each replica...")

	// Shut down
	for _, r := range replicas {
		killReplica(ts, r)
	}
}
