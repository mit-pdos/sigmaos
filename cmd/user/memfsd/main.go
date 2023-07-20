package main

import (
	"os"

	"sigmaos/container"
	db "sigmaos/debug"
	"sigmaos/memfssrv"
	"sigmaos/proc"
	"sigmaos/sigmasrv"
	"sigmaos/repldummy"
	sp "sigmaos/sigmap"
)

func main() {
	name := sp.MEMFS + "/" + proc.GetPid().String()
	if len(os.Args) > 1 {
		ip, err := container.LocalIP()
		if err != nil {
			db.DFatalf("Error get local ip: %v", err)
		}
		addr := ip + ":0"
		config := repldummy.MakeConfig()
		if os.Args[1] == "dummy" {
			mfs, err := memfssrv.MakeReplMemFs(addr, name, sp.Tuname("memfsd-"+proc.GetPid().String()), config, proc.GetRealm())
			if err != nil {
				db.DFatalf("Error makreplmemfs: %v", err)
			}
			mfs.Serve()
			mfs.Exit(proc.MakeStatus(proc.StatusEvicted))
		}
	} else {
		ssrv, err := sigmasrv.MakeSigmaSrv(name, nil, sp.Tuname(name))
		if err != nil {
			db.DFatalf("MakeSigmaSrv %v\n", err)
		}
		ssrv.Serve()
		db.DPrintf(db.TEST, "evicted\n")
		ssrv.Exit(proc.MakeStatus(proc.StatusEvicted))
	}
}
