package machine

import (
	"path"

	db "ulambda/debug"
	"ulambda/fslib"
	np "ulambda/ninep"
	"ulambda/resource"
)

// Post chunks of available cores.
func PostCores(fsl *fslib.FsLib, machineId string, cores *np.Tinterval) {
	// Post cores in local fs.
	if _, err := fsl.PutFile(path.Join(MACHINES, machineId, CORES, cores.String()), 0777, np.OWRITE, []byte(cores.String())); err != nil {
		db.DFatalf("Error PutFile: %v", err)
	}
	msg := resource.MakeResourceMsg(resource.Tgrant, resource.Tcore, cores.String(), 1)
	if _, err := fsl.SetFile(np.SIGMACTL, msg.Marshal(), np.OWRITE, 0); err != nil {
		db.DFatalf("Error SetFile in PostCores: %v", err)
	}
}
