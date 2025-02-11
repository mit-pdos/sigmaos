package proc

import (
	"fmt"
	"io"
	"sigmaos/proc"
	mschedclnt "sigmaos/sched/msched/clnt"
	"sigmaos/shell/shellctx"
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
	realm := ctx.Tstate.ProcEnv().GetRealm()
	runningProcsInRealm := make([]*proc.Proc, 0)

	for _, k := range ctx.Tstate.Kclnts {
		sdc := mschedclnt.NewMSchedClnt(ctx.Tstate.FsLib, k.KernelId())
		runningProcsInKernel, err := sdc.GetRunningProcs(1)
		if err != nil {
			fmt.Fprintf(stderr, "Error getting running procs for kernel:%v %v\n", err, k.KernelId())
		}
		runningProcsInRealm = append(runningProcsInRealm, runningProcsInKernel[realm]...)
	}
	for _, proc := range runningProcsInRealm {
		fmt.Fprintf(stdout, "id:%v dir:%v mem: %v, cpu: %v, type:%v, kernel id:%v \n", proc.GetPid(), proc.GetParentDir(), proc.GetMem(), proc.GetMcpu(), proc.GetType(), proc.GetKernelID())
	}
	return true
}
