package bootkernelclnt

import (
	"fmt"
	"os"
	"time"

	"sigmaos/container"
	"sigmaos/fslib"
	"sigmaos/kernelclnt"
	"sigmaos/proc"
	"sigmaos/procclnt"
	"sigmaos/serr"
	sp "sigmaos/sigmap"
)

//
// Library to start a kernel boot process.
//

const (
	HOME      = "/home/sigmaos"
	ROOTREALM = "rootrealm"
)

type Kernel struct {
	*fslib.FsLib
	*procclnt.ProcClnt
	kclnt     *kernelclnt.KernelClnt
	container *container.Container
}

var envvar = []string{proc.SIGMADEBUG, proc.SIGMAPERF}

// Takes named port and returns filled-in namedAddr
func BootKernelNamed(yml string, nameds []string) (*Kernel, []string, error) {
	k, err := startContainer(yml, nameds)
	if err != nil {
		return nil, nil, err
	}
	nds, err := fslib.SetNamedIP(k.GetIP(), nameds)
	if err != nil {
		return nil, nil, err
	}
	k, err = k.waitUntilBooted(nds)
	if err != nil {
		return nil, nil, err
	}
	return k, nds, err
}

// Takes filled-in namedAddr
func BootKernel(yml string, nameds []string) (*Kernel, error) {
	k, err := startContainer(yml, nameds)
	if err != nil {
		return nil, err
	}
	return k.waitUntilBooted(nameds)
}

func (k *Kernel) Boot(s string) error {
	return k.kclnt.Boot(s, []string{})
}

func (k *Kernel) KillOne(s string) error {
	return k.kclnt.Kill(s)
}

func (k *Kernel) GetIP() string {
	return k.container.Ip()
}

func (k *Kernel) GetClnt() (*fslib.FsLib, *procclnt.ProcClnt) {
	return k.FsLib, k.ProcClnt
}

func (k *Kernel) MkClnt(name string, namedAddr []string) (*fslib.FsLib, *procclnt.ProcClnt, error) {
	fsl, err := fslib.MakeFsLibAddr(name, k.container.Ip(), namedAddr)
	if err != nil {
		return nil, nil, err
	}
	pclnt := procclnt.MakeProcClntInit(proc.GenPid(), fsl, name, namedAddr)
	return fsl, pclnt, nil
}

func (k *Kernel) Shutdown() error {
	k.kclnt.Shutdown()
	return k.container.Shutdown()
}

func startContainer(yml string, nameds []string) (*Kernel, error) {
	container, err := container.StartKContainer(yml, nameds, makeEnv())
	if err != nil {
		return nil, err
	}
	return &Kernel{container: container}, nil
}

func (k *Kernel) waitUntilBooted(nameds []string) (*Kernel, error) {
	const N = 100
	for i := 0; i < N; i++ {
		time.Sleep(10 * time.Millisecond)
		fsl, pclnt, err := k.MkClnt("kclnt", nameds)
		if err == nil {
			k.FsLib = fsl
			k.ProcClnt = pclnt
			break
		} else if serr.IsErrUnavailable(err) {
			fmt.Printf(".")
			continue
		} else {
			return nil, err
		}
	}
	for i := 0; i < N; i++ {
		time.Sleep(10 * time.Millisecond)
		kclnt, err := kernelclnt.MakeKernelClnt(k.FsLib, sp.BOOT+"~local/")
		if err == nil {
			k.kclnt = kclnt
			fmt.Printf("running\n")
			break
		} else if serr.IsErrUnavailable(err) {
			fmt.Printf(".")
		} else {
			return nil, err
		}
	}
	if k.kclnt == nil {
		return nil, fmt.Errorf("BootKernel: timeded out")
	}
	return k, nil
}

func makeEnv() []string {
	env := []string{}
	for _, s := range envvar {
		if e := os.Getenv(s); e != "" {
			env = append(env, fmt.Sprintf("%s=%s", s, e))
		}
	}
	env = append(env, fmt.Sprintf("%s=%s", proc.SIGMAREALM, ROOTREALM))
	return env
}
