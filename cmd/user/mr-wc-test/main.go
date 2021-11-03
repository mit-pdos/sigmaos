package main

//
// Run in ulambda top-level directory
//

import (
	"bytes"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	// "path"
	"strconv"
	"strings"

	"ulambda/fslib"
	"ulambda/mr"
	np "ulambda/ninep"
	"ulambda/proc"
	"ulambda/procinit"
	"ulambda/realm"
)

func Compare(fsl *fslib.FsLib) {
	cmd := exec.Command("sort", "mr/seq-mr.out")
	var out1 bytes.Buffer
	cmd.Stdout = &out1
	err := cmd.Run()
	if err != nil {
		log.Printf("cmd err %v\n", err)
	}
	cmd = exec.Command("sort", "mr/par-mr.out")
	var out2 bytes.Buffer
	cmd.Stdout = &out2
	err = cmd.Run()
	if err != nil {
		log.Printf("cmd err %v\n", err)
	}
	b1 := out1.Bytes()
	b2 := out2.Bytes()
	if len(b1) != len(b2) {
		log.Fatalf("Output files have different length\n")
	}
	for i, v := range b1 {
		if v != b2[i] {
			log.Fatalf("Buf %v diff %v %v\n", i, v, b2[i])
			break
		}
	}
}

func main() {
	fsl1 := fslib.MakeFsLib("mr-wc-1")
	cfg := realm.GetRealmConfig(fsl1, realm.TEST_RID)

	fsl := fslib.MakeFsLibAddr("mr-wc-test", cfg.NamedAddr)
	procinit.SetProcLayers(map[string]bool{procinit.PROCBASE: true, procinit.PROCDEP: true})
	sclnt := procinit.MakeProcClntInit(fsl, procinit.GetProcLayersMap(), cfg.NamedAddr)

	if err := fsl.Mkdir(mr.MRDIR, 0777); err != nil {
		log.Fatalf("Mkdir %v\n", err)
	}

	if err := fsl.Mkdir(mr.MDIR, 0777); err != nil {
		log.Fatalf("Mkdir %v\n", err)
	}

	if err := fsl.Mkdir(mr.RDIR, 0777); err != nil {
		log.Fatalf("Mkdir %v\n", err)
	}

	if err := fsl.Mkdir(mr.MCLAIM, 0777); err != nil {
		log.Fatalf("Mkdir %v\n", err)
	}

	if err := fsl.Mkdir(mr.RCLAIM, 0777); err != nil {
		log.Fatalf("Mkdir %v\n", err)
	}

	// input directories for reduce tasks
	for r := 0; r < mr.NReduce; r++ {
		n := "name/mr/r/" + strconv.Itoa(r)
		if err := fsl.Mkdir(n, 0777); err != nil {
			log.Fatalf("Mkdir %v err %v\n", n, err)
		}
	}

	// Put names of input files in name/mr/m
	files, err := ioutil.ReadDir("input/")
	if err != nil {
		log.Fatalf("Readdir %v\n", err)
	}
	for _, f := range files {
		// remove mapper output directory from previous run
		fsl.RmDir("name/ux/~ip/m-" + f.Name())
		n := mr.MDIR + "/" + f.Name()
		if _, err := fsl.PutFile(n, []byte(n), 0777, np.OWRITE); err != nil {
			log.Fatalf("PutFile %v err %v\n", n, err)
		}
	}

	// Start workers
	workers := map[string]bool{}
	for i := 0; i < mr.NWorker; i++ {
		pid := proc.GenPid()
		a := proc.MakeProc(pid, "bin/user/worker", []string{"bin/user/mr-m-wc",
			"bin/user/mr-r-wc"})
		sclnt.Spawn(a)
		workers[pid] = true
	}

	// Wait for workers to exit
	for w, _ := range workers {
		status, err := sclnt.WaitExit(w)
		if err != nil && !strings.Contains(err.Error(), "file not found") || status != "OK" && status != "" {
			log.Fatalf("Wait failed %v, %v\n", err, status)
		}
	}

	file, err := os.OpenFile("mr/par-mr.out", os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		log.Fatalf("Couldn't open output file\n")
	}

	defer file.Close()
	for i := 0; i < mr.NReduce; i++ {
		// XXX run as a lambda?
		r := strconv.Itoa(i)
		data, err := fsl.ReadFile(mr.ROUT + r)
		if err != nil {
			log.Fatalf("ReadFile %v err %v\n", r, err)
		}
		_, err = file.Write(data)
		if err != nil {
			log.Fatalf("Write err %v\n", err)
		}
	}

	Compare(fsl)
	log.Printf("mr-wc PASS\n")
}
