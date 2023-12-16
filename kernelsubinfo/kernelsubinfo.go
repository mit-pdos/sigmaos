package kernelsubinfo

import (
	"path"

	db "sigmaos/debug"
	"sigmaos/fslib"
	"sigmaos/proc"
	sp "sigmaos/sigmap"
)

const (
	SUBSYSTEM_INFO = "subsystem-info"
)

type SubsystemInfo struct {
	Kpid sp.Tpid
	Addr *sp.Taddr
}

func NewSubsystemInfo(kpid sp.Tpid, addr *sp.Taddr) *SubsystemInfo {
	return &SubsystemInfo{kpid, addr}
}

func RegisterSubsystemInfo(fsl *fslib.FsLib, si *SubsystemInfo) {
	if err := fsl.PutFileJson(path.Join(proc.PROCDIR, SUBSYSTEM_INFO), 0777, si); err != nil {
		db.DFatalf("PutFileJson (%v): %v", path.Join(proc.PROCDIR, SUBSYSTEM_INFO), err)
	}
}

func GetSubsystemInfo(fsl *fslib.FsLib, kpids string, pid string) *SubsystemInfo {
	si := &SubsystemInfo{}
	if err := fsl.GetFileJson(path.Join(kpids, pid, SUBSYSTEM_INFO), si); err != nil {
		db.DFatalf("Error GetFileJson in subsystem info: %v", err)
		return nil
	}
	return si
}
