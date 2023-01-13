package bootkernelclnt

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"

	// sc "sigmaos/container"

	"sigmaos/fslib"
	"sigmaos/kernelclnt"
	"sigmaos/proc"
	"sigmaos/procclnt"
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
	namedAddr []string
	ip        string
	cli       *client.Client
	container string
}

var envvar = []string{proc.SIGMADEBUG, proc.SIGMAPERF, proc.SIGMANAMED}
var image string

func init() {
	flag.StringVar(&image, "image", "", "docker image")
}

func BootKernel(yml string) (*Kernel, error) {
	k, err := bootKernel(yml)
	if err != nil {
		return nil, err
	}
	nameds, err := fslib.SetNamedIP(k.ip)
	if err != nil {
		return nil, err
	}
	k.namedAddr = nameds
	log.Printf("nameds %v\n", nameds)
	fsl, pclnt, err := mkClient(k.ip, ROOTREALM, nameds)
	if err != nil {
		return nil, err
	}
	k.FsLib = fsl
	k.ProcClnt = pclnt
	kclnt, err := kernelclnt.MakeKernelClnt(fsl, sp.BOOT+"~local/")
	if err != nil {
		return nil, err
	}
	k.kclnt = kclnt
	return k, nil
}

func (k *Kernel) KillOne(s string) error {
	return k.kclnt.Kill(s)
}

func (k *Kernel) NamedAddr() []string {
	return k.namedAddr
}

func (k *Kernel) GetIP() string {
	return k.ip
}

func (k *Kernel) Boot(s string) error {
	return k.kclnt.Boot(s)
}

func (k *Kernel) Shutdown() error {
	ctx := context.Background()
	out, err := k.cli.ContainerLogs(ctx, k.container, types.ContainerLogsOptions{ShowStderr: true})
	if err != nil {
		panic(err)
	}
	stdcopy.StdCopy(os.Stdout, os.Stderr, out)
	return k.cli.ContainerKill(ctx, k.container, "SIGTERM")
}

func bootKernel(yml string) (*Kernel, error) {
	ctx := context.Background()
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}
	log.Printf("create container from image %v %v\n", image, yml)

	env := makeEnv()
	resp, err := cli.ContainerCreate(ctx, &container.Config{
		Image: image,
		Cmd:   []string{"bin/linux/bootkernel", yml},
		//AttachStdout: true,
		// AttachStderr: true,
		Tty: true,
		Env: env,
	}, nil, nil, nil, "")
	if err := cli.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
		return nil, err
	}
	json, err1 := cli.ContainerInspect(ctx, resp.ID)
	if err1 != nil {
		return nil, err
	}
	ip := json.NetworkSettings.IPAddress
	log.Printf("container %v  running at %v\n", resp.ID[:10], ip)
	time.Sleep(10 * time.Second) // XXX fix
	return &Kernel{ip: ip, cli: cli, container: resp.ID}, nil
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

func mkClient(kip string, realmid string, namedAddr []string) (*fslib.FsLib, *procclnt.ProcClnt, error) {
	fsl, err := fslib.MakeFsLibAddr("test", kip, namedAddr)
	if err != nil {
		return nil, nil, err
	}
	pclnt := procclnt.MakeProcClntInit(proc.GenPid(), fsl, "test", namedAddr)
	return fsl, pclnt, nil
}
