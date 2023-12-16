package sigmaclntsrv_test

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"

	db "sigmaos/debug"
	"sigmaos/frame"
	"sigmaos/rpc"
	"sigmaos/rpcclnt"
	// "sigmaos/serr"
	scproto "sigmaos/sigmaclntsrv/proto"
	// sp "sigmaos/sigmap"
)

type RPCCh struct {
	req io.Writer
	rep io.Reader
}

func (rpcch *RPCCh) WriteRead(a []byte) ([]byte, error) {
	if err := frame.WriteFrame(rpcch.req, a); err != nil {
		db.DPrintf(db.ALWAYS, "WriteFrame err %v\n", err)
		return nil, err
	}
	b, r := frame.ReadFrame(rpcch.rep)
	if r != nil {
		return nil, r
	}
	return b, nil
}

func (rpcch *RPCCh) StatsSrv() (*rpc.SigmaRPCStats, error) {
	return nil, nil
}

func TestStat(t *testing.T) {
	cmd := exec.Command("../bin/linux/sigmaclntd", []string{}...)
	stdin, err := cmd.StdinPipe()
	assert.Nil(t, err)
	stdout, err := cmd.StdoutPipe()
	assert.Nil(t, err)
	cmd.Stderr = os.Stderr

	err = cmd.Start()
	assert.Nil(t, err)

	req := scproto.StatRequest{Path: "name/"}
	rep := scproto.StatReply{}

	rpcch := &RPCCh{stdin, stdout}
	rpcc, err := rpcclnt.NewRPCClntCh(rpcch)
	assert.Nil(t, err)
	rpcc.RPC("SigmaClntSrv.Stat", &req, &rep)

	fmt.Printf("rep: %v\n", rep)

	cmd.Wait()
}
