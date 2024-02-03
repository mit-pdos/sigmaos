package sessclnt_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"sigmaos/ctx"
	db "sigmaos/debug"
	"sigmaos/dir"
	"sigmaos/ephemeralmap"
	"sigmaos/frame"
	"sigmaos/memfs"
	"sigmaos/netsrv"
	"sigmaos/path"
	"sigmaos/proc"
	"sigmaos/protsrv"
	"sigmaos/serr"
	"sigmaos/sessclnt"
	"sigmaos/sessp"
	"sigmaos/sesssrv"
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

type Tstate struct {
	lip  sp.Tip
	pcfg *proc.ProcEnv
	addr *sp.Taddr
}

func NewTstate() *Tstate {
	lip := sp.Tip("127.0.0.1")
	pcfg := proc.NewTestProcEnv(sp.ROOTREALM, lip, lip, lip, "", false, false)
	pcfg.Program = "srv"
	pcfg.SetUname("srv")
	addr := sp.NewTaddr(sp.NO_IP, sp.INNER_CONTAINER_IP, 1110)
	proc.SetSigmaDebugPid(pcfg.GetPID().String())
	return &Tstate{lip: lip, pcfg: pcfg, addr: addr}
}

func TestConnectSessSrv(t *testing.T) {
	ts := NewTstate()
	ss := &SessSrv{}

	srv := netsrv.NewNetServer(ts.pcfg, ss, ts.addr)
	db.DPrintf(db.TEST, "srv %v\n", srv.MyAddr())

	smgr := sessclnt.NewMgr(sp.ROOTREALM.String())
	req := sp.NewTattach(0, sp.NoFid, "clnt", 0, path.Path{})
	rep, err := smgr.RPC(sp.Taddrs{srv.MyAddr()}, req, nil)
	assert.Nil(t, err)
	db.DPrintf(db.TEST, "fcall %v\n", rep)
	srv.CloseListener()
}

func TestConnectMfsSrv(t *testing.T) {
	ts := NewTstate()
	et := ephemeralmap.NewEphemeralMap()
	root := dir.NewRootDir(ctx.NewCtxNull(), memfs.NewInode, nil)
	srv := sesssrv.NewSessSrv(ts.pcfg, root, ts.addr, protsrv.NewProtServer, et, nil)

	smgr := sessclnt.NewMgr(sp.ROOTREALM.String())
	req := sp.NewTattach(0, sp.NoFid, "clnt", 0, path.Path{})
	rep, err := smgr.RPC(sp.Taddrs{srv.MyAddr()}, req, nil)
	assert.Nil(t, err)
	db.DPrintf(db.TEST, "fcall %v\n", rep)
}
