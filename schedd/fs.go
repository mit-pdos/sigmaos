package schedd

import (
	"path"

	db "sigmaos/debug"
	"sigmaos/memfssrv"
	"sigmaos/proc"
	sp "sigmaos/sigmap"
)

func (sd *Schedd) postProcInQueue(p *proc.Proc) {
	if _, err := sd.mfs.Create(path.Join(sp.QUEUE, p.GetPid().String()), 0777, sp.OWRITE); err != nil {
		db.DFatalf("Error create %v: %v", p.GetPid(), err)
	}
}

func (sd *Schedd) removeProcFromQueue(p *proc.Proc) {
	if err := sd.mfs.Remove(path.Join(sp.QUEUE, p.GetPid().String())); err != nil {
		db.DFatalf("Error remove %v: %v", p.GetPid(), err)
	}
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
