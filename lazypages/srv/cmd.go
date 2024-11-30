package srv

import (
	"io"
	"os"
	"os/exec"

	db "sigmaos/debug"
	"sigmaos/proc"
	sp "sigmaos/sigmap"
)

type LazyPagesSrvCmd struct {
	p   *proc.Proc
	cmd *exec.Cmd
	out io.WriteCloser
}

func ExecLazyPagesSrv(p *proc.Proc, innerIP sp.Tip, outerIP sp.Tip, uprocdpid sp.Tpid) (*LazyPagesSrvCmd, error) {
	p.FinalizeEnv(innerIP, outerIP, uprocdpid)
	db.DPrintf(db.LAZYPAGESSRV, "ExecLazyPagesSrv: %v", p)
	cmd := exec.Command("lazypagesd")
	cmd.Env = p.GetEnv()
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		db.DPrintf(db.LAZYPAGESSRV, "Error start %v %v", cmd, err)
		return nil, err
	}
	buf := make([]byte, 1)
	if _, err := io.ReadFull(stdout, buf); err != nil {
		db.DPrintf(db.LAZYPAGESSRV, "read pipe err %v\n", err)
		return nil, err
	}
	db.DPrintf(db.ALWAYS, "lazypagesd %v started", uprocdpid)
	return &LazyPagesSrvCmd{
		p:   p,
		cmd: cmd,
		out: stdin,
	}, nil
}

func (bsc *LazyPagesSrvCmd) Shutdown() error {
	if _, err := io.WriteString(bsc.out, "e"); err != nil {
		return err
	}
	return nil
}
