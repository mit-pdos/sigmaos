package example_echo_server

import (
	dbg "sigmaos/debug"
	"sigmaos/fs"
	"sigmaos/proc"
	"sigmaos/rand"
	sp "sigmaos/sigmap"
	"sigmaos/sigmasrv"
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
	echosrv := &EchoSrv{rand.String(8)}
	dbg.DPrintf(DEBUG_ECHO_SERVER, "==%v== Creating echo server \n", echosrv.sid)
	ssrv, err := sigmasrv.NewSigmaSrv(NAMED_ECHO_SERVER, echosrv, proc.GetProcEnv())
	if err != nil {
		return err
	}
	dbg.DPrintf(DEBUG_ECHO_SERVER, "==%v== Starting to run echo service\n", echosrv.sid)
	return ssrv.RunServer()
}

// find meaning of life for request
func (echosrv *EchoSrv) Echo(ctx fs.CtxI, req EchoRequest, rep *EchoResult) error {
	dbg.DPrintf(DEBUG_ECHO_SERVER, "==%v== Received Echo Request: %v\n", echosrv.sid, req)
	rep.Text = req.Text
	return nil
}
