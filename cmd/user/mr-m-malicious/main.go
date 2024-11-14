package main

import (
	"path"

	db "sigmaos/debug"
	"sigmaos/proc"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
)

func main() {
	sc, err := sigmaclnt.NewSigmaClnt(proc.GetProcEnv())
	if err != nil {
		db.DFatalf("Error NewSigmaClnt: %v", err)
	}
	if err := sc.Started(); err != nil {
		db.DFatalf("Error Started: %v", err)
	}
	// Should be able to access the restricted MR bucket. If not, then exit with
	// OK so that the output files don't match.
	if _, err := sc.GetDir(path.Join(sp.S3, sp.LOCAL, "mr-restricted")); err != nil {
		sc.ClntExitOK()
	}
	if sts, err := sc.GetDir(path.Join(sp.S3, sp.LOCAL, "9ps3")); err == nil {
		// If able to get access to the s3 bucket the malicious mapper shouldn't
		// have access to, declare success (from the malicious mapper's
		// perspective)
		db.DPrintf(db.ERROR, "HAHAHA! I can access your restricted S3 bucket [%v] ;)\n%v", path.Join(sp.S3, sp.LOCAL, "9ps3"), sp.Names(sts))
		sc.ClntExitOK()
	} else {
		db.DPrintf(db.ALWAYS, "ARGH! Malicious mapper foiled!: %v", err)
		sc.ClntExit(proc.NewStatusErr(err.Error(), nil))
	}
}
