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

	"ulambda/fslib"
	"ulambda/mr"
	"ulambda/proc"
)

func RmDir(dir string) error {
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer d.Close()
	names, err := d.Readdirnames(-1)
	if err != nil {
		return err
	}
	for _, name := range names {
		err = os.RemoveAll(path.Join(dir, name))
		if err != nil {
			return err
		}
	}
	err = os.RemoveAll(dir)
	if err != nil {
		return err
	}
	return nil
}

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
	fsl := fslib.MakeFsLib("mr-wc")
	pctl := proc.MakeProcCtl(fsl)
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
		pid1 := fslib.GenPid()
		pid2 := fslib.GenPid()
		m := strconv.Itoa(n)
		rmDir(fsl, "name/ux/~ip/m-"+m)
		a1 := &fslib.Attr{pid1, "bin/fsreader", "",
			[]string{"name/s3/~ip/input/" + f.Name(), m}, nil,
			[]fslib.PDep{fslib.PDep{pid1, pid2}}, nil, 0, fslib.T_BE,
			fslib.C_DEF}
		a2 := &fslib.Attr{pid2, "bin/mr-m-wc", "",
			[]string{"name/" + m + "/pipe", m}, nil,
			[]fslib.PDep{fslib.PDep{pid1, pid2}}, nil, 0, fslib.T_BE,
			fslib.C_DEF}
		pctl.Spawn(a1)
		pctl.Spawn(a2)
		n += 1
		mappers[pid2] = false
	}

	reducers := []string{}
	for i := 0; i < mr.NReduce; i++ {
		pid := fslib.GenPid()
		r := strconv.Itoa(i)
		a := &fslib.Attr{pid, "bin/mr-r-wc", "",
			[]string{"name/fs/" + r, "name/fs/mr-out-" + r}, nil,
			nil, mappers, 0, fslib.T_BE, fslib.C_DEF}
		reducers = append(reducers, pid)
		pctl.Spawn(a)
	}

	// Spawn noop lambda that is dependent on reducers
	pid := fslib.GenPid()
	pctl.SpawnNoOp(pid, reducers)
	status, err := pctl.Wait(pid)
	if err != nil {
		log.Fatalf("Wait failed %v status %v\n", err, string(status))
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
