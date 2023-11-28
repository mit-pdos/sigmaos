package schedd

import (
	//db "sigmaos/debug"
	"sigmaos/memfssrv"
	"sigmaos/procclnt"
	//sp "sigmaos/sigmap"
)

func setupMemFsSrv(mfs *memfssrv.MemFs) {
	procclnt.MountPids(mfs.SigmaClnt().FsLib)
}
