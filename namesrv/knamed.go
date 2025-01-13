package namesrv

import (
	"fmt"
	"io/ioutil"
	"os"

	db "sigmaos/debug"
	dialproxyclnt "sigmaos/dialproxy/clnt"
	"sigmaos/proc"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
	"sigmaos/util/perf"
)

func RunKNamed(args []string) error {
	pe := proc.GetProcEnv()
	db.DPrintf(db.NAMED, "%v: knamed %v", pe.GetPID(), args)
	if len(args) != 3 {
		db.DFatalf("%v: wrong number of arguments %v", args[0], args)
	}
	nd := newNamed(sp.Trealm(args[1]))

	p, err := perf.NewPerf(pe, perf.KNAMED)
	if err != nil {
		db.DFatalf("Error NewPerf: %v", err)
	}
	defer p.Done()

	sc, err := sigmaclnt.NewSigmaClntFsLib(pe, dialproxyclnt.NewDialProxyClnt(pe))
	if err != nil {
		db.DFatalf("NewSigmaClntFsLib: err %v", err)
	}
	nd.SigmaClnt = sc

	// Allow connections from all realms, so that realms can mount the kernel
	// service union directories
	nd.GetDialProxyClnt().AllowConnectionsFromAllRealms()

	init := args[2]

	db.DPrintf(db.NAMED, "started %v %v", pe.GetPID(), nd.realm)

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

	ep, err := nd.newSrv()
	if err != nil {
		db.DFatalf("Error newSrv %v\n", err)
	}

	db.DPrintf(db.NAMED, "newSrv %v ep %v", nd.realm, ep)

	nd.SigmaSrv.Mount(sp.PSTATSD, nd.pstats)

	if err := nd.fs.SetRootNamed(ep); err != nil {
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
	defer r.Close()

	db.DPrintf(db.NAMED, "%v: knamed done %v %v %v\n", pe.GetPID(), nd.realm, ep, string(data))

	nd.resign()

	return nil
}

var InitRootDir = []string{sp.BOOT, sp.KPIDS, sp.MEMFS, sp.LCSCHED, sp.BESCHED, sp.MSCHED, sp.UX, sp.S3, sp.DB, sp.MONGO, sp.REALM, sp.CHUNKD}

// If initial root dir doesn't exist, create it.
func (nd *Named) initfs() error {
	for _, n := range InitRootDir {
		if _, err := nd.SigmaClnt.Create(n, 0777|sp.DMDIR, sp.OREAD); err != nil {
			db.DPrintf(db.ALWAYS, "Error create [%v]: %v", n, err)
			return err
		}
	}
	return nil
}
