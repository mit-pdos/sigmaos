package machine

import (
	"path"

	db "sigmaos/debug"
	"sigmaos/fslib"
	"sigmaos/linuxsched"
	"sigmaos/resource"
	np "sigmaos/sigmap"
)

func NodedNCores() uint64 {
	n := uint64(np.Conf.Machine.CORE_GROUP_FRACTION * float64(linuxsched.NCores))
	// Make sure the minimum noded size is 1.
	if n < 1 {
		n = 1
	}
	return n
}

// Post chunks of available cores.
func PostCores(fsl *fslib.FsLib, machineId string, cores *np.Tinterval) {
	// Post cores in local fs.
	if _, err := fsl.PutFile(path.Join(MACHINES, machineId, CORES, cores.Marshal()), 0777, np.OWRITE, []byte(cores.Marshal())); err != nil {
		db.DFatalf("Error PutFile: %v", err)
	}
	msg := resource.MakeResourceMsg(resource.Tgrant, resource.Tcore, cores.Marshal(), 1)
	resource.SendMsg(fsl, np.SIGMACTL, msg)
}
