package schedd

import (
	db "sigmaos/debug"
	"sigmaos/memfssrv"
	"sigmaos/procclnt"
	sp "sigmaos/sigmap"
)

func setupMemFsSrv(mfs *memfssrv.MemFs) {
	mfs.GetStats().DisablePathCnts()
	procclnt.MountPids(mfs.SigmaClnt().FsLib, mfs.SigmaClnt().NamedAddr())
}

// Setup schedd's fs.
func setupFs(mfs *memfssrv.MemFs, sd *Schedd) {
	dirs := []string{
		sp.QUEUE,
		sp.RUNNING,
		sp.PIDS,
	}
	for _, d := range dirs {
		if _, err := mfs.Create(d, sp.DMDIR|0777, sp.OWRITE); err != nil {
			db.DFatalf("Error create %v: %v", d, err)
		}
	}
}
