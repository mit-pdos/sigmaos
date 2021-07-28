package sync

import (
	"ulambda/fslib"
)

var fsl *fslib.FsLib

func Init(fl *fslib.FsLib) {
	fsl = fl
}
