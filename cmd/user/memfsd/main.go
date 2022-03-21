package main

import (
	"log"
	"os"

	"ulambda/fidclnt"
	"ulambda/fslibsrv"
	"ulambda/linuxsched"
	np "ulambda/ninep"
	"ulambda/proc"
	"ulambda/repldummy"
)

func main() {
	linuxsched.ScanTopology()
	name := np.MEMFS + "/" + proc.GetPid().String()
	if len(os.Args) > 1 {
		ip, err := fidclnt.LocalIP()
		if err != nil {
			log.Fatalf("Error get local ip: %v", err)
		}
		addr := ip + ":0"
		config := repldummy.MakeConfig()
		if os.Args[1] == "dummy" {
			fss, err := fslibsrv.MakeReplMemFs(addr, name, "memfsd-"+proc.GetPid().String(), config)
			if err != nil {
				log.Fatalf("FATAL Error makreplmemfs: %v", err)
			}
			fss.Serve()
			fss.Done()
		}
	} else {
		mfs, _, _, err := fslibsrv.MakeMemFs(name, name)
		if err != nil {
			log.Fatalf("FATAL MakeMemFs %v\n", err)
		}
		mfs.Serve()
		mfs.Done()
	}
}
