package main

import (
	"os"

	"sigmaos/container"
	db "sigmaos/debug"
	"sigmaos/memfssrv"
	"sigmaos/proc"
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
			fss, err := memfssrv.MakeReplMemFs(addr, name, "memfsd-"+proc.GetPid().String(), config, proc.GetRealm())
			if err != nil {
				db.DFatalf("Error makreplmemfs: %v", err)
			}
			fss.Serve()
			fss.Done()
		}
	} else {
		mfs, _, err := memfssrv.MakeMemFs(name, name)
		if err != nil {
			db.DFatalf("MakeMemFs %v\n", err)
		}
		mfs.Serve()
		mfs.Done()
	}
}
