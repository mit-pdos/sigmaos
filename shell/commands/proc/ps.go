package proc

import (
	"fmt"
	"io"
	"sigmaos/shell/shellctx"
	mschedclnt "sigmaos/sched/msched/clnt"
	sp "sigmaos/sigmap"
)

type PsCommand struct{}

func NewPsCommand() *PsCommand {
	return &PsCommand{}
}

func (c *PsCommand) Name() string {
	return "ps"
}

func (c *PsCommand) Usage() string {
	return "ps"
}

func (c *PsCommand) Execute(ctx *shellctx.ShellContext, args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) bool {
	if len(args) != 0 {
		fmt.Fprintf(stderr, "Invalid number of arguments\n %v", c.Usage())
		return false
	}
	for _, k := range ctx.Tstate.Kclnts {
		fmt.Println(k.KernelId())
	}
	sdc := mschedclnt.NewMSchedClnt(ctx.Tstate.FsLib, ctx.Tstate.Kclnts[0].KernelId())
	ans, _ := sdc.GetRunningProcs(sp.Conf.Realm.N_SAMPLE)
	fmt.Println(ans)
	sts, _ := ctx.Tstate.GetDir("name/kpids")

	for _, filename := range sp.Names(sts) {
		fmt.Fprintln(stdout, filename)
	}
	return true
}
