package sigmaclntsrv_test

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"testing"

	"google.golang.org/protobuf/proto"

	"github.com/stretchr/testify/assert"

	db "sigmaos/debug"
	"sigmaos/frame"
	rpcproto "sigmaos/rpc/proto"
	"sigmaos/serr"
	scproto "sigmaos/sigmaclntsrv/proto"
	sp "sigmaos/sigmap"
)

type RPCClnt struct {
	req io.Writer
	rep io.Reader
}

func (rpcc *RPCClnt) rpc(method string, a []byte) (*rpcproto.Reply, error) {
	req := rpcproto.Request{Method: method, Args: a}

	b, err := proto.Marshal(&req)
	if err != nil {
		return nil, serr.NewErrError(err)
	}
	db.DPrintf(db.ALWAYS, "Req %v\n", req)
	if err := frame.WriteFrame(rpcc.req, b); err != nil {
		db.DPrintf(db.ALWAYS, "WriteFrame err %v\n", err)
		return nil, err
	}

	b, r := frame.ReadFrame(rpcc.rep)
	if r != nil {
		return nil, err
	}

	rep := &rpcproto.Reply{}
	if err := proto.Unmarshal(b, rep); err != nil {
		db.DPrintf(db.ALWAYS, "Unmarshall err %v\n", err)
		return nil, serr.NewErrError(err)
	}
	db.DPrintf(db.ALWAYS, "Rep %v\n", rep)
	return rep, nil
}

func (rpcc *RPCClnt) RPC(method string, arg proto.Message, res proto.Message) error {
	b, err := proto.Marshal(arg)
	if err != nil {
		return err
	}
	rep, sr := rpcc.rpc(method, b)
	if sr != nil {
		return err
	}
	if rep.Err.ErrCode != 0 {
		return sp.NewErr(rep.Err)
	}
	if err := proto.Unmarshal(rep.Res, res); err != nil {
		return err
	}
	return nil
}

func TestConnect(t *testing.T) {
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

	rpcc := &RPCClnt{stdin, stdout}
	rpcc.RPC("SigmaClntSrv.Stat", &req, &rep)

	fmt.Printf("rep: %v\n", rep)

	cmd.Wait()
}
