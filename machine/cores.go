package machine

import (
	"path"

	db "sigmaos/debug"
	"sigmaos/fslib"
	np "sigmaos/ninep"
	"sigmaos/resource"
)

// Post chunks of available cores.
func PostCores(fsl *fslib.FsLib, machineId string, cores *np.Tinterval) {
	// Post cores in local fs.
	if _, err := fsl.PutFile(path.Join(MACHINES, machineId, CORES, cores.String()), 0777, np.OWRITE, []byte(cores.String())); err != nil {
		db.DFatalf("Error PutFile: %v", err)
	}
	msg := resource.MakeResourceMsg(resource.Tgrant, resource.Tcore, cores.String(), 1)
	resource.SendMsg(fsl, np.SIGMACTL, msg)
}
