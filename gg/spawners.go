package gg

import (
	//	"fmt"
	//	"log"
	//	"path"

	"sigmaos/proc"
)

// Given a PID, create a no-op which waits on that Pid
func spawnNoOp(launch ExecutorLauncher, waitPid string) string {
	//	noOpPid := noOpPid(waitPid)
	//	exitDep := []string{waitPid}
	// XXX no more no-ops
	//	err := launch.SpawnNoOp(noOpPid, exitDep)
	//	if err != nil {
	//		db.DFatalf("Error spawning noop [%v]: %v\n", noOpPid, err)
	//	}
	return ""
}

func spawnOrigDirUploader(launch ExecutorLauncher, dir string, subDir string) string {
	//	a := procdep.MakeProcDep(origDirUploaderPid(subDir), "user/fsdiruploader", []string{
	//		ggOrig(dir, subDir, ""),
	//		ggRemote(subDir, ""),
	//		"",
	//	})
	//	err := launch.Spawn(a)
	//	if err != nil {
	//		db.DFatalf("Error spawning orig dir upload worker [%v/%v]: %v\n", dir, subDir, err)
	//	}
	//	return a.Pid
	return ""
}

func spawnReductionWriter(launch ExecutorLauncher, target string, targetReduction string, dstDir string, subDir string, deps []string) proc.Tpid {
	//	a := procdep.MakeProcDep(reductionWriterPid(dstDir, subDir, target), "user/gg-target-writer", []string{
	//		path.Join(dstDir, subDir),
	//		target,
	//		targetReduction,
	//	})
	//	reductionPid := outputHandlerPid(targetReduction)
	//	noOpReductionPid := noOpPid(reductionPid)
	//	deps = append(deps, noOpReductionPid)
	//	exitDepMap := map[string]bool{}
	//	for _, dep := range deps {
	//		exitDepMap[dep] = false
	//	}
	//	a.Dependencies = &procdep.Deps{nil, exitDepMap}
	//	err := launch.Spawn(a)
	//	if err != nil {
	//		db.DFatalf("Error spawning target writer [%v]: %v\n", target, err)
	//	}
	//	return a.Pid
	return ""
}

func spawnExecutor(launch ExecutorLauncher, targetHash string, depPids []proc.Tpid) (proc.Tpid, error) {
	//	a := procdep.MakeProcDep(executorPid(targetHash), "user/gg-executor", []string{
	//		targetHash,
	//	})
	//	a.Dependencies = &procdep.Deps{map[string]bool{}, map[string]bool{}}
	//	exitDepMap := map[string]bool{}
	//	for _, dep := range depPids {
	//		exitDepMap[dep] = false
	//	}
	//	a.Dependencies.ExitDep = exitDepMap
	//	err := launch.Spawn(a)
	//	if err != nil {
	//		return a.Pid, fmt.Errorf("Error spawning executor [%v]: %v\n", targetHash, err)
	//	}
	return "", nil
}

func spawnThunkOutputHandler(launch ExecutorLauncher, deps []string, thunkHash string, outputFiles []string) string {
	//	args := []string{
	//		thunkHash,
	//	}
	//	args = append(args, outputFiles...)
	//	a := procdep.MakeProcDep(outputHandlerPid(thunkHash), "user/gg-thunk-output-handler", args)
	//	exitDepMap := map[string]bool{}
	//	for _, dep := range deps {
	//		exitDepMap[dep] = false
	//	}
	//	a.Dependencies = &procdep.Deps{nil, exitDepMap}
	//	err := launch.Spawn(a)
	//	if err != nil {
	//		db.DFatalf("Error spawning output handler [%v]: %v\n", thunkHash, err)
	//	}
	return ""
}
