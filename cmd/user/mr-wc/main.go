package main

//
// Run in ulambda top-level directory
//

import (
	"log"
	"os"
	"path"
	"strconv"
	"strings"

	"ulambda/fslib"
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

func main() {
	if len(os.Args) < 3 {
		log.Fatalf("Usage: %v nReducers s3_input_dir", os.Args[0])
	}
	nReducers, err := strconv.Atoi(os.Args[1])
	if err != nil {
		log.Fatalf("Error invalid nReducers: %v", err)
	}
	s3Dir := os.Args[2]

	fsl1 := fslib.MakeFsLib("mr-wc-1")
	cfg := realm.GetRealmConfig(fsl1, realm.TEST_RID)
	fsl := fslib.MakeFsLibAddr("mr-wc", cfg.NamedAddr)
	procinit.SetProcLayers(map[string]bool{procinit.PROCBASE: true, procinit.PROCDEP: true})
	sclnt := procinit.MakeProcClnt(fsl, procinit.GetProcLayersMap())
	for r := 0; r < nReducers; r++ {
		s := strconv.Itoa(r)
		err := fsl.Mkdir("name/fs/"+s, 0777)
		if err != nil {
			log.Fatalf("Mkdir %v\n", err)
		}
	}

	mappers := map[string]bool{}
	n := 0
	files, err := fsl.ReadDir(path.Join("name/s3/~ip/", s3Dir))
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
		//			[]string{m, path.Join("name/s3/~ip/", s3Dir, f.Name)},
		//			[]string{procinit.GetProcLayersString()},
		//			proc.T_BE, proc.C_DEF,
		//		}
		a2 := procdep.MakeProcDep(pid2, "bin/user/mr-m-wc", []string{path.Join("name/s3/~ip/", s3Dir, f.Name), m})
		a2.Dependencies = &procdep.Deps{map[string]bool{}, nil}
		a2.Env = []string{procinit.GetProcLayersString()}
		a2.Proc.Type = proc.T_BE
		//		sclnt.Spawn(a1)
		sclnt.Spawn(a2)
		n += 1
		mappers[pid2] = false
	}

	reducers := []string{}
	for i := 0; i < nReducers; i++ {
		pid := proc.GenPid()
		r := strconv.Itoa(i)
		a := procdep.MakeProcDep(pid, "bin/user/mr-r-wc", []string{"name/fs/" + r, "name/fs/mr-out-" + r})
		a.Proc.Env = []string{procinit.GetProcLayersString()}
		a.Proc.Type = proc.T_BE
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
	for i := 0; i < nReducers; i++ {
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

	log.Printf("mr-wc DONE\n")
}
