package example_echo_server

import (
	"sigmaos/api/fs"
	dbg "sigmaos/debug"
	"sigmaos/example/example_echo_server/proto"
	"sigmaos/proc"
	sp "sigmaos/sigmap"
	"sigmaos/sigmasrv"
	"sigmaos/util/rand"
)

// YH:
// Toy server echoing request message

type EchoSrv struct {
	sid string
}

const DEBUG_ECHO_SERVER = "ECHO_SERVER"
const DIR_ECHO_SERVER = sp.NAMED + "example/"
const NAMED_ECHO_SERVER = DIR_ECHO_SERVER + "echo-server"

func RunEchoSrv() error {
	echosrv := &EchoSrv{rand.Name()}
	dbg.DPrintf(DEBUG_ECHO_SERVER, "==%v== Creating echo server \n", echosrv.sid)
	ssrv, err := sigmasrv.NewSigmaSrv(NAMED_ECHO_SERVER, echosrv, proc.GetProcEnv())
	if err != nil {
		return err
	}
	dbg.DPrintf(DEBUG_ECHO_SERVER, "==%v== Starting to run echo service\n", echosrv.sid)
	return ssrv.RunServer()
}

// find meaning of life for request
func (echosrv *EchoSrv) Echo(ctx fs.CtxI, req proto.EchoReq, rep *proto.EchoRep) error {
	dbg.DPrintf(DEBUG_ECHO_SERVER, "==%v== Received Echo Request: %v\n", echosrv.sid, req)
	rep.Text = req.Text
	return nil
}
