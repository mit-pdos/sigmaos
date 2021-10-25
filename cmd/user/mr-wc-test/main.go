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
	"path"
	"strconv"
	"strings"

	"ulambda/fslib"
	"ulambda/mr"
	"ulambda/proc"
	"ulambda/procdep"
	"ulambda/procinit"
	"ulambda/realm"
)

func rmDir(fsl *fslib.FsLib, dir string) error {
	fs, err := fsl.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, f := range fs {
		fsl.Remove(path.Join(dir, f.Name))
	}
	fsl.Remove(dir)
	return nil
}

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
	fsl := fslib.MakeFsLibAddr("mr-wc", cfg.NamedAddr)
	procinit.SetProcLayers(map[string]bool{procinit.PROCBASE: true, procinit.PROCDEP: true})
	sclnt := procinit.MakeProcClnt(fsl, procinit.GetProcLayersMap())
	for r := 0; r < mr.NReduce; r++ {
		s := strconv.Itoa(r)
		err := fsl.Mkdir("name/fs/"+s, 0777)
		if err != nil {
			log.Fatalf("Mkdir %v\n", err)
		}
	}

	mappers := map[string]bool{}
	n := 0
	files, err := ioutil.ReadDir("input/")
	if err != nil {
		log.Fatalf("Readdir %v\n", err)
	}
	for _, f := range files {
		//		pid1 := proc.GenPid()
		pid2 := proc.GenPid()
		m := strconv.Itoa(n)
		rmDir(fsl, "name/ux/~ip/m-"+m)
		//		a1 := procdep.MakeProcDep()
		//		a1.Dependencies = &procdep.Deps{map[string]bool{}, nil}
		//		a1.Proc = &proc.Proc{pid1, "bin/user/fsreader", "",
		//			[]string{m, "name/s3/~ip/input/" + f.Name()},
		//			[]string{procinit.GetProcLayersString()},
		//			proc.T_BE, proc.C_DEF,
		//		}
		a2 := procdep.MakeProcDep(pid2, "bin/user/mr-m-wc", []string{"name/s3/~ip/input/" + f.Name(), m})
		a2.Env = []string{procinit.GetProcLayersString()}
		a2.Type = proc.T_BE
		a2.Ncore = proc.C_DEF
		a2.Dependencies = &procdep.Deps{map[string]bool{}, nil}
		//		sclnt.Spawn(a1)
		sclnt.Spawn(a2)
		n += 1
		mappers[pid2] = false
	}

	reducers := []string{}
	for i := 0; i < mr.NReduce; i++ {
		pid := proc.GenPid()
		r := strconv.Itoa(i)
		a := procdep.MakeProcDep(pid, "bin/user/mr-r-wc", []string{"name/fs/" + r, "name/fs/mr-out-" + r})
		a.Env = []string{procinit.GetProcLayersString()}
		a.Type = proc.T_BE
		a.Ncore = proc.C_DEF
		a.Dependencies = &procdep.Deps{nil, mappers}
		reducers = append(reducers, pid)
		sclnt.Spawn(a)
	}

	// Wait for reducers to exit
	for _, r := range reducers {
		status, err := sclnt.WaitExit(r)
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
		data, err := fsl.ReadFile("name/fs/mr-out-" + r)
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
