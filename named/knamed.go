package named

import (
	"fmt"
	"io/ioutil"
	"os"

	db "sigmaos/debug"
	"sigmaos/proc"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
)

func RunKNamed(args []string) error {
	pcfg := proc.GetProcEnv()
	db.DPrintf(db.NAMED, "%v: knamed %v\n", pcfg.GetPID(), args)
	if len(args) != 3 {
		return fmt.Errorf("%v: wrong number of arguments %v", args[0], args)
	}
	nd := &Named{}
	nd.realm = sp.Trealm(args[1])

	sc, err := sigmaclnt.MkSigmaClntFsLib(pcfg)
	if err != nil {
		db.DFatalf("MkSigmaClntFsLib: err %v", err)
	}
	nd.SigmaClnt = sc

	init := args[2]

	db.DPrintf(db.NAMED, "started %v %v %v\n", pcfg.GetPID(), nd.realm, proc.GetRealm())

	w := os.NewFile(uintptr(3), "pipew")
	r := os.NewFile(uintptr(4), "piper")
	w2 := os.NewFile(uintptr(5), "pipew")
	w2.Close()

	if init == "start" {
		fmt.Fprintf(w, init)
		w.Close()
	}

	if err := nd.startLeader(); err != nil {
		db.DFatalf("Error startLeader %v\n", err)
	}
	defer nd.fs.Close()

	if err := nd.mkSrv(); err != nil {
		db.DFatalf("Error mkSrv %v\n", err)
	}

	mnt := sp.MkMountServer(nd.MyAddr())
	if err := nd.fs.SetRootNamed(mnt); err != nil {
		db.DFatalf("SetNamed: %v", err)
	}

	if init == "init" {
		nd.initfs()
		fmt.Fprintf(w, init)
		w.Close()
	}

	data, err := ioutil.ReadAll(r)
	if err != nil {
		db.DPrintf(db.ALWAYS, "pipe read err %v", err)
		return err
	}
	r.Close()

	db.DPrintf(db.NAMED, "%v: knamed done %v %v %v\n", pcfg.GetPID(), nd.realm, mnt, string(data))

	nd.resign()

	return nil
}

var InitRootDir = []string{sp.BOOT, sp.KPIDS, sp.SCHEDD, sp.UX, sp.S3, sp.DB}

// If initial root dir doesn't exist, create it.
func (nd *Named) initfs() error {
	// XXX clean up WS here for now
	if err := nd.RmDir(sp.WS); err != nil {
		db.DPrintf(db.ALWAYS, "Failed to clean up %v err %v", sp.WS, err)
	}
	for _, n := range InitRootDir {
		if _, err := nd.SigmaClnt.Create(n, 0777|sp.DMDIR, sp.OREAD); err != nil {
			db.DPrintf(db.ALWAYS, "Error create [%v]: %v", n, err)
			return err
		}
	}
	return nil
}
