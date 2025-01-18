package fslib

import (
	"bufio"
	"io"
	"os"
	"path/filepath"

	db "sigmaos/debug"
	sp "sigmaos/sigmap"
)

// Upload lpn into sigma a spn
func (fsl *FsLib) UploadFile(lpn, spn string) error {
	src, err := os.OpenFile(lpn, os.O_RDONLY, 0)
	if err != nil {
		return err
	}
	defer src.Close()
	rdr := bufio.NewReader(src)
	wrt, err := fsl.CreateWriter(spn, 0777, sp.OWRITE)
	if err != nil {
		db.DPrintf(db.FSLIB, "OpenWriter %v err %v\n", spn, err)
		return err
	}
	defer wrt.Close()
	if _, err := io.Copy(wrt, rdr); err != nil {
		db.DPrintf(db.FSLIB, "UploadFile: Copy err %v", err)
		return err
	}
	return nil
}

// Upload lpn dir into sigma at spn
func (fsl *FsLib) UploadDir(lpn, spn string) error {
	fsl.MkDir(spn, 0777)
	files, err := os.ReadDir(lpn)
	if err != nil {
		db.DPrintf(db.FSLIB, "ReadDir %v %v", lpn, err)
		return err
	}
	for _, file := range files {
		if err := fsl.UploadFile(filepath.Join(lpn, file.Name()), filepath.Join(spn, file.Name())); err != nil {
			db.DPrintf(db.FSLIB_ERR, "UploadFile %v err %v\n", file.Name(), err)
			return err
		}
	}
	return nil
}

// Download a sigma a into lpn
func (fsl *FsLib) DownloadFile(spn, lpn string) error {
	src, err := fsl.OpenReader(spn)
	if err != nil {
		db.DPrintf(db.FSLIB, "OpenWriter %v err %v\n", spn, err)
		return err
	}
	defer src.Close()
	rdr := bufio.NewReader(src)
	// wrt, err := os.OpenFile(lpn, os.O_RDWR, 0)
	wrt, err := os.OpenFile(lpn, os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return err
	}
	defer wrt.Close()
	if _, err := io.Copy(wrt, rdr); err != nil {
		db.DPrintf(db.FSLIB, "UploadFile: Copy err %v", err)
		return err
	}
	return nil
}
