package sessclnt_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	db "sigmaos/debug"
	"sigmaos/frame"
	"sigmaos/netsrv"
	"sigmaos/path"
	"sigmaos/proc"
	"sigmaos/serr"
	"sigmaos/sessclnt"
	"sigmaos/sessp"
	sp "sigmaos/sigmap"
	"sigmaos/spcodec"
)

type SessSrv struct {
}

func (ss *SessSrv) ReportError(err error) {
}

func (ss *SessSrv) ServeRequest(req []frame.Tframe) ([]frame.Tframe, *serr.Err) {
	fc0 := spcodec.UnmarshalFcallAndData(req[0], req[1])
	db.DPrintf(db.TEST, "fcall %v\n", fc0)
	msg := &sp.Rattach{Qid: sp.NewQidPerm(0777, 0, 0)}
	fc1 := sessp.NewFcallMsgReply(fc0, msg)
	fc1.Data = nil
	rep := spcodec.MarshalFcallWithoutData(fc1)
	return []frame.Tframe{rep, fc1.Data}, nil
}

func TestConnect(t *testing.T) {
	lip := sp.Tip("127.0.0.1")
	pcfg := proc.NewTestProcEnv(sp.ROOTREALM, lip, lip, lip, "", false, false)
	pcfg.Program = "srv"
	pcfg.SetUname("srv")
	addr := sp.NewTaddr(sp.NO_IP, sp.INNER_CONTAINER_IP, 1110)
	proc.SetSigmaDebugPid(pcfg.GetPID().String())

	ss := &SessSrv{}

	srv := netsrv.NewNetServer(pcfg, ss, addr, spcodec.WriteFcallAndData, spcodec.ReadUnmarshalFcallAndData)
	db.DPrintf(db.TEST, "srv %v\n", srv.MyAddr())

	smgr := sessclnt.NewMgr(sp.ROOTREALM.String())
	req := sp.NewTattach(0, sp.NoFid, "clnt", 0, path.Path{})
	rep, err := smgr.RPC(sp.Taddrs{srv.MyAddr()}, req, nil)
	assert.Nil(t, err)
	db.DPrintf(db.TEST, "fcall %v\n", rep)
}
