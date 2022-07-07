package benchmarks

import (
	"io/ioutil"
	"log"
	"strconv"
	"time"

	db "ulambda/debug"
	"ulambda/fslib"
	"ulambda/group"
	"ulambda/groupmgr"
	"ulambda/kv"
	"ulambda/mr"
	np "ulambda/ninep"
	"ulambda/proc"
	"ulambda/procclnt"
	"ulambda/rand"
)

/*
 * Realm balance benchmark.
 *
 * - Goal: Show that we can effectively and efficienlty multiplex resources
 *   across multiple competing tenants running LC and BE tasks.
 * - Setup: Realm 1000 is executing a long-runnning MR job (BE) and realm arielck
 *   is executing a long-running KV job (LC). As the load on the KV service
 *   grows, we want our system to dynamically reallocate resources from the MR
 *   to the KV job so as to keep KV client latency low. Once the KV service's
 *   load decreases, we would like to see resources shift back to the MR job.
 */

type RealmBalanceBenchmark struct {
	fsl1       *fslib.FsLib
	pclnt1     *procclnt.ProcClnt
	namedAddr1 []string
	fsl2       *fslib.FsLib
	pclnt2     *procclnt.ProcClnt
	namedAddr2 []string
	resDir     string
	mr         *groupmgr.GroupMgr
}

func MakeRealmBalanceBenchmark(fsl1 *fslib.FsLib, namedAddr1 []string, fsl2 *fslib.FsLib, namedAddr2 []string, resDir string) *RealmBalanceBenchmark {
	r := &RealmBalanceBenchmark{}
	r.fsl1 = fsl1
	r.fsl2 = fsl2
	r.resDir = resDir
	pid := proc.GenPid()
	r.pclnt1 = procclnt.MakeProcClntInit(pid, r.fsl1, "realm-balance-1", namedAddr1)
	r.pclnt2 = procclnt.MakeProcClntInit(pid, r.fsl2, "realm-balance-2", namedAddr2)
	r.namedAddr1 = namedAddr1
	r.namedAddr2 = namedAddr2
	return r
}

func setupMR(fsl *fslib.FsLib, job string) int {
	files, err := ioutil.ReadDir("./input/")
	if err != nil {
		db.DFatalf("Readdir %v\n", err)
	}
	// XXX out of date
	for _, f := range files {
		n := mr.MapTask(job) + "/" + f.Name()
		if _, err := fsl.PutFile(n, 0777, np.OWRITE, []byte(n)); err != nil {
			db.DFatalf("PutFile %v err %v\n", n, err)
		}
	}
	return len(files)
}

func StartMR(fsl *fslib.FsLib, pclnt *procclnt.ProcClnt) *groupmgr.GroupMgr {
	const NReduce = 2
	crashtask := 0
	crashcoord := 0
	job := rand.String(16)

	// Set up the fs & job
	mr.InitCoordFS(fsl, job, NReduce)
	nmap := setupMR(fsl, job)

	return groupmgr.Start(fsl, pclnt, mr.NCOORD, "user/mr-coord", []string{job, strconv.Itoa(nmap), strconv.Itoa(NReduce), "user/mr-m-wc", "user/mr-r-wc", strconv.Itoa(crashtask)}, mr.NCOORD, crashcoord, 0, 0)
}

func balancerOp(fsl *fslib.FsLib, opcode, mfs string) error {
	for true {
		err := kv.BalancerOp(fsl, opcode, mfs)
		if err == nil {
			return nil
		}
		if np.IsErrUnavailable(err) || np.IsErrRetry(err) {
			time.Sleep(100 * time.Millisecond)
		} else {
			db.DPrintf(db.ALWAYS, "balancer op err %v", err)
			return err
		}
	}
	return nil
}

func setupKV(namedAddrs []string, fsl *fslib.FsLib, pclnt *procclnt.ProcClnt, nclerk, repl, ncrash, nkeys int, mfsgrps *[]*groupmgr.GroupMgr) *kv.KvClerk {
	// Create first shard group
	gn := group.GRP + "0"
	grp := kv.SpawnGrp(fsl, pclnt, gn, repl, ncrash)
	err := balancerOp(fsl, "add", gn)
	*mfsgrps = append(*mfsgrps, grp)

	// Create keys
	clrk, err := kv.MakeClerkFsl(fsl, pclnt)
	if err != nil {
		db.DFatalf("Error make clerk: %v", err)
	}
	for i := uint64(0); i < uint64(nkeys); i++ {
		err := clrk.Put(kv.MkKey(i), []byte{})
		if err != nil {
			db.DFatalf("Error clerk put: %v", err)
		}
	}
	return clrk
}

func StartKV(namedAddrs []string, fsl *fslib.FsLib, pclnt *procclnt.ProcClnt) {
	nBalancer := 3
	nReplicas := 0
	crashhelper := "0"
	nclerk := 1
	ncrash := 0
	nkeys := 100
	crashbal := 0
	auto := "manual"
	mfsgrps := []*groupmgr.GroupMgr{}
	gmbal := groupmgr.Start(fsl, pclnt, nBalancer, "user/balancer", []string{crashhelper, auto}, nBalancer, crashbal, 0, 0)
	setupKV(namedAddrs, fsl, pclnt, nclerk, nReplicas, ncrash, nkeys, &mfsgrps)
	//	for i := 0; i < nclerk; i++ {
	//		pid := ts.startClerk()
	//		ts.clrks = append(ts.clrks, pid)
	//	}
	//
	//	for s := 0; s < NKV; s++ {
	//		grp := group.GRP + strconv.Itoa(s+1)
	//		gm := SpawnGrp(ts.FsLib, ts.ProcClnt, grp, repl, ncrash)
	//		ts.mfsgrps = append(ts.mfsgrps, gm)
	//		err := ts.balancerOp("add", grp)
	//		assert.Nil(ts.T, err, "BalancerOp")
	//		// do some puts/gets
	//		time.Sleep(TIME * time.Millisecond)
	//	}
	//
	//	for s := 0; s < NKV; s++ {
	//		grp := group.GRP + strconv.Itoa(len(ts.mfsgrps)-1)
	//		err := ts.balancerOp("del", grp)
	//		assert.Nil(ts.T, err, "BalancerOp")
	//		ts.mfsgrps[len(ts.mfsgrps)-1].Stop()
	//		ts.mfsgrps = ts.mfsgrps[0 : len(ts.mfsgrps)-1]
	//		// do some puts/gets
	//		time.Sleep(TIME * time.Millisecond)
	//	}
	//
	//	ts.stopClerks()

	gmbal.Stop()

	mfsgrps[0].Stop()
}

func (rb *RealmBalanceBenchmark) Run() map[string]*RawResults {
	log.Printf("=== RUN    RealmBalanceBenchmark")
	r := make(map[string]*RawResults)
	db.DPrintf("TEST", "Starting MR")
	rb.mr = StartMR(rb.fsl1, rb.pclnt1)
	db.DPrintf("TEST", "Starting KV")
	StartKV(rb.namedAddr2, rb.fsl2, rb.pclnt2)
	db.DPrintf("TEST", "Finished KV")
	rb.mr.Wait()
	db.DPrintf("TEST", "Finished MR")
	log.Printf("--- PASS: RealmBalanceBenchmark")
	return r
}
