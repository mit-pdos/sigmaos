package perf

import (
	"log"
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	"ulambda/fsclnt"
	"ulambda/fslib"
	np "ulambda/ninep"
)

const (
	PORT_OFFSET    = 30001
	CONFIG_PATH    = "name/configPath"
	UNION_DIR_PATH = "name/memfs-replica"
)

type MemfsReplicationTest struct {
	unionDirPath string
	headAddr     string
	headPath9p   string
	nSrv         int
	nCli         int
	secs         int
	repl         bool
	*fslib.FsLib
}

func MakeMemfsReplicationTest(args []string) *MemfsReplicationTest {
	if len(args) < 4 {
		log.Fatalf("MemfsdReplicaTest perf insufficient args: %v", args)
	}

	t := &MemfsReplicationTest{}
	nCli, err := strconv.Atoi(args[0])
	if err != nil {
		log.Fatalf("Error converting ncli: %v, %v", args[0], err)
	}
	secs, err := strconv.Atoi(args[1])
	if err != nil {
		log.Fatalf("Error converting secs: %v, %v", args[1], err)
	}
	repl, err := strconv.ParseBool(args[2])
	if err != nil {
		log.Fatalf("Error converting bool: %v, %v", args[2], err)
	}
	nSrv, err := strconv.Atoi(args[3])
	if err != nil {
		log.Fatalf("Error converting nsrv: %v, %v", args[3], err)
	}
	ip, err := fsclnt.LocalIP()
	if err != nil {
		log.Fatalf("Couldn't get local ip from fsclnt: %v", err)
	}
	t.headAddr = ip + ":30001"
	if repl {
		t.headPath9p = path.Join(UNION_DIR_PATH, t.headAddr)
	} else {
		t.headPath9p = path.Join("name", "fs")
	}
	t.nSrv = nSrv
	t.nCli = nCli
	t.secs = secs
	t.repl = repl
	t.FsLib = fslib.MakeFsLib("MemfsReplPerfTest")
	return t
}

func (t *MemfsReplicationTest) createCliFiles() {
	for i := 0; i < t.nCli; i++ {
		i_str := strconv.Itoa(i)
		err := t.MakeFile(path.Join(t.headPath9p, i_str), 0777, np.OWRITE, []byte("abcd"))
		if err != nil {
			log.Fatalf("Error creating file for client: %v", err)
		}
	}
}

func (t *MemfsReplicationTest) setupUnionDir() {
	err := t.Mkdir(UNION_DIR_PATH, 0777)
	if err != nil {
		log.Fatalf("Couldn't make union dir: %v", err)
	}
}

func (t *MemfsReplicationTest) Work() {
	N := t.nSrv

	replicas := t.allocReplicas(N)
	t.writeConfig(replicas)
	t.setupUnionDir()

	// Start up
	for _, r := range replicas {
		bootReplica(r)
	}

	t.createCliFiles()

	time.Sleep(1000 * time.Millisecond)

	var start sync.WaitGroup
	var end sync.WaitGroup

	start.Add(t.nCli + 1)
	end.Add(t.nCli)

	done := false
	opcounts := make([]int, t.nCli)
	times := make([]float64, t.nCli)

	// Write some files to the head
	log.Printf("Starting clients...")
	for id := 0; id < t.nCli; id++ {
		go t.basicClient(id, &done, opcounts, times, &start, &end)
	}

	log.Printf("Waiting for clients to terminate...")
	start.Done()
	start.Wait()
	tStart := time.Now()
	for {
		if time.Since(tStart).Seconds() >= float64(t.secs) {
			done = true
			break
		}
	}
	end.Wait()
	log.Printf("Done waiting")
	totOps := float64(0.0)
	totTime := float64(0.0)
	for i := 0; i < t.nCli; i++ {
		totOps += float64(opcounts[i])
		totTime += float64(times[i])
	}
	avgLatency := totTime / totOps
	avgTpt := totOps / totTime
	log.Printf("time: %v", times)
	log.Printf("ops: %v", opcounts)
	log.Printf("Latency: %f (sec/op)", avgLatency)
	log.Printf("Tpt: %f (op/sec)", avgTpt)
}

func (t *MemfsReplicationTest) basicClient(id int, done *bool, opcounts []int, times []float64, start *sync.WaitGroup, end *sync.WaitGroup) {
	defer end.Done()

	fsl := fslib.MakeFsLib("client-" + strconv.Itoa(id))
	id_str := strconv.Itoa(id)
	start.Done()
	start.Wait()
	tStart := time.Now()
	oc := 0
	for !*done {
		oc += 1
		_, err := fsl.ReadFile(path.Join(t.headPath9p, id_str))
		if err != nil {
			log.Fatalf("Error writing file in client: %v", err)
		}
	}
	times[id] = time.Since(tStart).Seconds()
	opcounts[id] = oc
}

type Replica struct {
	addr    string
	port    string
	crashed bool
	cmd     *exec.Cmd
}

func (t *MemfsReplicationTest) allocReplicas(n int) []*Replica {
	replicas := make([]*Replica, n)
	ip, err := fsclnt.LocalIP()
	if err != nil {
		log.Fatalf("Error getting ip: %v")
	}
	for i, _ := range replicas {
		portstr := strconv.Itoa(PORT_OFFSET + i)
		replicas[i] = &Replica{ip + ":" + portstr, portstr, false, nil}
	}
	return replicas
}

func (t *MemfsReplicationTest) writeConfig(replicas []*Replica) {
	addrs := []string{}
	for _, r := range replicas {
		addrs = append(addrs, r.addr)
	}
	config := strings.Join(addrs, "\n")
	err := t.MakeFile(CONFIG_PATH, 0777, np.OWRITE, []byte(config))
	if err != nil {
		log.Printf("Error making config file: %v", err)
	}
}

func run(bin string, name string, args []string) (*exec.Cmd, error) {
	cmd := exec.Command(bin+"/"+name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ())
	return cmd, cmd.Start()
}

func bootReplica(replica *Replica) {
	bin := "."
	var err error
	replica.cmd, err = run(bin,
		"bin/kernel/memfs-replica",
		[]string{"placeholder-pid",
			replica.port,
			CONFIG_PATH,
			UNION_DIR_PATH})
	if err != nil {
		log.Fatalf("Failed to boot replica: %v", err)
	}
	time.Sleep(100 * time.Millisecond)
}

func killReplica(replica *Replica) {
	err := replica.cmd.Process.Kill()
	if err != nil {
		log.Fatalf("Failed to kill replica: %v", err)
	}
	time.Sleep(100 * time.Millisecond)
}
