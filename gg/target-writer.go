package gg

import (
	"path"

	db "ulambda/debug"
	"ulambda/fslib"
	"ulambda/proc"
	"ulambda/procclnt"
)

// XXX Rename
type TargetWriter struct {
	pid             string
	cwd             string
	target          string
	targetReduction string
	*fslib.FsLib
	proc.ProcClnt
}

func MakeTargetWriter(args []string, debug bool) (*TargetWriter, error) {
	db.DPrintf("TargetWriter: %v\n", args)
	tw := &TargetWriter{}

	tw.pid = args[0]
	tw.cwd = args[1]
	tw.target = args[2]
	tw.targetReduction = args[3]
	fls := fslib.MakeFsLib("gg-target-writer")
	tw.FsLib = fls
	tw.ProcClnt = procclnt.MakeProcClnt(fls)
	tw.Started(tw.pid)
	return tw, nil
}

func (tw *TargetWriter) Exit() {
	tw.Exited(tw.pid, "OK")
}

func (tw *TargetWriter) Work() {
	// Read the final output's hash from the reducton file
	targetHash := getReductionResult(tw, tw.targetReduction)

	// Preserve the target name if target == reduction
	if tw.target == tw.targetReduction {
		tw.target = targetHash
	}

	// Copy to target location
	copyRemoteFile(tw, ggRemoteBlobs(targetHash), path.Join(tw.cwd, tw.target))
}

func (tw *TargetWriter) Name() string {
	return "TargetWriter " + tw.pid + " "
}
