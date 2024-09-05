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
