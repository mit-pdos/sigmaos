package namesrv

import (
	"fmt"
	"io/ioutil"
	"os"

	"github.com/golang-jwt/jwt"

	"sigmaos/auth"
	db "sigmaos/debug"
	"sigmaos/netproxyclnt"
	"sigmaos/perf"
	"sigmaos/proc"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
)

func RunKNamed(args []string) error {
	pe := proc.GetProcEnv()
	db.DPrintf(db.NAMED, "%v: knamed %v\n", pe.GetPID(), args)
	if len(args) != 5 {
		return fmt.Errorf("%v: wrong number of arguments %v", args[0], args)
	}
	// Since knamed is the first "host" of the realm namespace to start up, no
	// one (even the kernel it is started by) can bootstrap keys for it. So,
	// just have it use the kernel's master keys. This should be ok, in theory,
	// because knamed is short-lived anyway, and is only really used to start up
	// the other services.
	masterPubKey, err := auth.NewPublicKey[*jwt.SigningMethodECDSA](jwt.SigningMethodES256, []byte(args[3]))
	if err != nil {
		db.DFatalf("Error NewPublicKey: %v", err)
	}
	masterPrivKey, err := auth.NewPrivateKey[*jwt.SigningMethodECDSA](jwt.SigningMethodES256, []byte(args[4]))
	if err != nil {
		db.DFatalf("Error NewPublicKey: %v", err)
	}
	// Self-sign token for bootstrapping purposes
	nd := &Named{}
	nd.realm = sp.Trealm(args[1])

	p, err := perf.NewPerf(pe, perf.KNAMED)
	if err != nil {
		db.DFatalf("Error NewPerf: %v", err)
	}
	defer p.Done()

	sc, err := sigmaclnt.NewSigmaClntFsLib(pe, netproxyclnt.NewNetProxyClnt(pe, nil))
	if err != nil {
		db.DFatalf("NewSigmaClntFsLib: err %v", err)
	}
	nd.SigmaClnt = sc

	init := args[2]

	nd.masterPublicKey = masterPubKey
	nd.masterPrivKey = masterPrivKey
	nd.pubkey = masterPubKey
	nd.privkey = masterPrivKey
	nd.signer = sp.Tsigner(nd.SigmaClnt.ProcEnv().GetKernelID())

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
	r.Close()

	db.DPrintf(db.NAMED, "%v: knamed done %v %v %v\n", pe.GetPID(), nd.realm, ep, string(data))

	nd.resign()

	return nil
}

var InitRootDir = []string{sp.BOOT, sp.KPIDS, sp.MEMFS, sp.LCSCHED, sp.PROCQ, sp.SCHEDD, sp.UX, sp.S3, sp.DB, sp.MONGO, sp.REALM, sp.KEYD, sp.CHUNKD}

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
