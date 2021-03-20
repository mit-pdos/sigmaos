package fslambda

import (
	"io/ioutil"
	"log"
	"path"
	"strings"

	db "ulambda/debug"
	"ulambda/fslib"
)

type DirUploader struct {
	pid       string
	src       string
	dest      string
	thunkHash string
	*fslib.FsLib
}

func MakeDirUploader(args []string, debug bool) (*DirUploader, error) {
	db.DPrintf("DirUploader: %v\n", args)
	up := &DirUploader{}
	up.pid = args[0]
	up.src = args[1]
	up.dest = args[2]
	up.thunkHash = args[3]
	// XXX Should I use a more descriptive uname?
	fls := fslib.MakeFsLib("dir-uploader")
	up.FsLib = fls
	up.Started(up.pid)
	return up, nil
}

func (up *DirUploader) Work() {
	db.DPrintf("Uploading dir [%v] to [%v]\n", up.src, up.dest)
	files, err := ioutil.ReadDir(up.src)
	if err != nil {
		log.Fatalf("Read upload dir error: %v\n", err)
	}
	for _, f := range files {
		// Only copy/overwrite outputs produced by this thunk if we're uploading reductions
		if path.Base(up.src) != "reductions" || strings.Contains(f.Name(), up.thunkHash) {
			srcPath := path.Join(up.src, f.Name())
			dstPath := path.Join(up.dest, f.Name())
			contents, err := ioutil.ReadFile(srcPath)
			if err != nil {
				log.Fatalf("Read upload dir file error[%v]: %v\n", srcPath, err)
			}
			// Try and make a new file if one doesn't exist, else overwrite
			_, err = up.FsLib.Stat(dstPath)
			if err != nil {
				db.DPrintf("Mkfile dir uploader [%v]\n", dstPath)
				// XXX Perms?
				err = up.FsLib.MakeFile(dstPath, contents)
				if err != nil {
					// XXX This only occurs if someone else has written the file since we
					// last checked if it existed. Since it isn't a reduction (by the
					// check in the big if statement), this is ok. The contents will be
					// identical. Should change this to an atomic rename operation at some
					// point, though.
					log.Printf("Couldn't make upload dir file %v: %v", dstPath, err)
				}
			} else {
				db.DPrintf("Already exists [%v]\n", dstPath)
				err = up.FsLib.WriteFile(dstPath, contents)
				if err != nil {
					// XXX This only occurs if someone else has written the file since we
					// last checked if it existed. Since it isn't a reduction (by the
					// check in the big if statement), this is ok. The contents will be
					// identical. Should change this to an atomic rename operation at some
					// point, though.
					log.Printf("Couldn't write uplaod dir file [%v]: %v\n", dstPath, err)
				}
			}
		}
	}
}

func (up *DirUploader) Exit() {
	up.Exiting(up.pid, "OK")
}
