package fslambda

import (
	"io/ioutil"
	"log"

	db "ulambda/debug"
	"ulambda/fslib"
	np "ulambda/ninep"
	"ulambda/procclnt"
)

type Uploader struct {
	pid  string
	src  string
	dest string
	*fslib.FsLib
	*procclnt.ProcClnt
}

func MakeUploader(args []string, debug bool) (*Uploader, error) {
	db.DPrintf("Uploader: %v\n", args)
	up := &Uploader{}
	up.pid = args[0]
	up.src = args[1]
	up.dest = args[2]
	// XXX Should I use a more descriptive uname?
	fls := fslib.MakeFsLib("uploader")
	up.FsLib = fls
	up.ProcClnt = procclnt.MakeProcClnt(fls)
	up.Started(up.pid)
	return up, nil
}

func (up *Uploader) Work() {
	db.DPrintf("Uploading [%v] to [%v]\n", up.src, up.dest)
	contents, err := ioutil.ReadFile(up.src)
	if err != nil {
		log.Fatalf("Read file error: %v\n", err)
	}
	err = up.FsLib.MakeFile(up.dest, 0777, np.OWRITE, contents)
	if err != nil {
		db.DPrintf("Overwriting file\n")
		err = up.FsLib.WriteFile(up.dest, contents)
		if err != nil {
			log.Fatalf("Couldn't overwrite file [%v]: %v\n", up.dest, err)
		}
	}
}

func (up *Uploader) Exit() {
	up.Exited(up.pid, "OK")
}
