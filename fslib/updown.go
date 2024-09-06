package fslib

import (
	"bufio"
	"io"
	"os"

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

// Download spn into local lpn
func (fsl *FsLib) DownloadFile(spn, lpn string) error {
	rdr, err := fsl.OpenReader(spn)
	if err != nil {
		db.DPrintf(db.FSLIB, "OpenReader %v err %v\n", spn, err)
		return err
	}
	file, err := os.OpenFile(lpn, os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		db.DPrintf(db.FSLIB, "OpenFile %v err %v", lpn, err)
		return err
	}
	wrt := bufio.NewWriter(file)
	if _, err := io.Copy(wrt, rdr.Reader); err != nil {
		db.DPrintf(db.FSLIB, "Copy err %v", err)
		return err
	}
	rdr.Close()
	file.Close()
	return nil
}
