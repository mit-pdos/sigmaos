package gg

import (
	"log"
	"path"
	"strings"

	db "ulambda/debug"
	"ulambda/fslib"
)

// XXX Rename
type TargetWriter struct {
	pid             string
	cwd             string
	target          string
	targetReduction string
	*fslib.FsLib
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
	tw.Started(tw.pid)
	return tw, nil
}

func (tw *TargetWriter) Exit() {
	tw.Exiting(tw.pid, "OK")
}

func (tw *TargetWriter) Work() {
	// Read the final output's hash from the reducton file
	targetHash := tw.readTargetHash()

	// Preserve the target name if target == reduction
	if tw.target == tw.targetReduction {
		tw.target = targetHash
	}

	// Download to target location
	downPid := tw.spawnDownloader(targetHash)
	exitDepSwaps := []string{
		tw.pid,
		downPid,
	}
	db.DPrintf("Updating exit dependencies for [%v]\n", tw.pid)
	err := tw.SwapExitDependency(exitDepSwaps)
	if err != nil {
		log.Fatalf("Couldn't swap exit dependencies %v: %v\n", exitDepSwaps, err)
	}
}

func (tw *TargetWriter) readTargetHash() string {
	reductionPath := ggRemoteReductions(tw.targetReduction)
	f, err := tw.ReadFile(reductionPath)
	if err != nil {
		log.Fatalf("Couldn't read target reduction [%v]: %v\n", reductionPath, err)
	}
	return strings.TrimSpace(string(f))
}

// XXX Should get rid of this, and/or blend it into the spawners file
func (tw *TargetWriter) spawnDownloader(targetHash string) string {
	a := fslib.Attr{}
	subDir := path.Base(path.Dir(tw.cwd))
	a.Pid = reductionDownloaderPid(targetHash, subDir, tw.target)
	a.Program = "bin/fsdownloader"
	a.Args = []string{
		ggRemoteBlobs(targetHash),
		path.Join(tw.cwd, tw.target),
	}
	a.Env = []string{}
	a.PairDep = []fslib.PDep{}
	a.ExitDep = []string{uploaderPid(tw.targetReduction, GG_BLOBS, tw.target)}
	err := tw.Spawn(&a)
	if err != nil {
		db.DPrintf("Error spawning download worker [%v]: %v\n", tw.target, err)
	}
	return a.Pid
}
