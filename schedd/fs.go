package schedd

import (
	db "sigmaos/debug"
	"sigmaos/memfssrv"
	"sigmaos/procclnt"
	sp "sigmaos/sigmap"
)

func setupMemFsSrv(mfs *memfssrv.MemFs) {
	procclnt.MountPids(mfs.SigmaClnt().FsLib)
}

// Setup schedd's fs.
func setupFs(mfs *memfssrv.MemFs) {
	dirs := []string{
		sp.RUNNING,
		sp.PIDS,
	}
	for _, d := range dirs {
		if _, err := mfs.Create(d, sp.DMDIR|0777, sp.OWRITE, sp.NoLeaseId); err != nil {
			db.DFatalf("Error create %v: %v", d, err)
		}
	}
}
