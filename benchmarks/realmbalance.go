package benchmarks

import (
	"ulambda/fslib"
	//	np "ulambda/ninep"
	//	"ulambda/perf"
	//	"ulambda/proc"
	"ulambda/procclnt"
	//	"ulambda/realm"
)

type RealmBalanceBenchmark struct {
	*fslib.FsLib
	*procclnt.ProcClnt
}

func MakRealmBalanceBenchmark() *RealmBalanceBenchmark {
	r := &RealmBalanceBenchmark{}
	return r
}
