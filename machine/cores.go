package machine

import (
	"path"

	db "sigmaos/debug"
	"sigmaos/sessp"
	"sigmaos/linuxsched"
	mproto "sigmaos/machine/proto"
	"sigmaos/protdevclnt"
	sp "sigmaos/sigmap"
)

func NodedNCores() uint64 {
	n := uint64(sp.Conf.Machine.CORE_GROUP_FRACTION * float64(linuxsched.NCores))
	// Make sure the minimum noded size is 1.
	if n < 1 {
		n = 1
	}
	return n
}

// Post chunks of available cores.
func PostCores(pdc *protdevclnt.ProtDevClnt, machineId string, cores *sessp.Tinterval) {
	// Post cores in local fs.
	if _, err := pdc.PutFile(path.Join(MACHINES, machineId, CORES, cores.Marshal()), 0777, sp.OWRITE, []byte(cores.Marshal())); err != nil {
		db.DFatalf("Error PutFile: %v", err)
	}
	res := &mproto.MachineResponse{}
	req := &mproto.MachineRequest{
		Ncores: 1,
	}
	err := pdc.RPC("SigmaMgr.FreeCores", req, res)
	if err != nil || !res.OK {
		db.DFatalf("Error RPC: %v %v", err, res.OK)
	}
}
