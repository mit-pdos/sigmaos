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
	k, err := startContainer(yml)
	if err != nil {
		return nil, err
	}
	nameds, err := fslib.SetNamedIP(k.ip, []string{":1111"})
	if err != nil {
		return nil, err
	}
	k.namedAddr = nameds
	log.Printf("named %v\n", nameds)
	fslib.SetSigmaNamed(nameds)
	return k.waitUntilBooted()
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

func startContainer(yml string) (*Kernel, error) {
	ctx := context.Background()
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}
	env := makeEnv()
	resp, err := cli.ContainerCreate(ctx, &container.Config{
		Image: image,
		Cmd:   []string{"bin/linux/bootkernel", yml},
		//AttachStdout: true,
		// AttachStderr: true,
		Tty: true,
		Env: env,
	}, &container.HostConfig{
		//Unnecessary with using docker for user containers.
		//CapAdd:      []string{"SYS_ADMIN"},
		//SecurityOpt: []string{"seccomp=unconfined"},
		//
		// This is bad idea in general because it requires to give rw
		// permission on host to privileged daemon.  But maybe ok in
		// our case where kernel is trusted as is. XXX Use different
		// image for user procs.
		Binds: []string{
			"/var/run/docker.sock:/var/run/docker.sock",
		},
	}, nil, nil, "")
	if err := cli.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
		return nil, err
	}
	json, err1 := cli.ContainerInspect(ctx, resp.ID)
	if err1 != nil {
		return nil, err
	}
	ip := json.NetworkSettings.IPAddress
	fmt.Printf("container %s with image %s booting at %s...\n", resp.ID[:10], image, ip)
	return &Kernel{ip: ip, cli: cli, container: resp.ID}, nil
}

func (k *Kernel) waitUntilBooted() (*Kernel, error) {
	const N = 100
	for i := 0; i < N; i++ {
		time.Sleep(10 * time.Millisecond)
		fsl, pclnt, err := mkClient(k.ip, ROOTREALM, k.namedAddr)
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

func mkClient(kip string, realmid string, namedAddr []string) (*fslib.FsLib, *procclnt.ProcClnt, error) {
	fsl, err := fslib.MakeFsLibAddr("test", kip, namedAddr)
	if err != nil {
		return nil, nil, err
	}
	pclnt := procclnt.MakeProcClntInit(proc.GenPid(), fsl, "test", namedAddr)
	return fsl, pclnt, nil
}
