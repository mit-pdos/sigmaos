package bootkernelclnt

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"

	// sc "sigmaos/container"

	db "sigmaos/debug"
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
	ip        string
	cli       *client.Client
	container string
}

var envvar = []string{proc.SIGMADEBUG, proc.SIGMAPERF}
var image string

func init() {
	flag.StringVar(&image, "image", "", "docker image")
}

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
	return k.kclnt.Boot(s)
}

func (k *Kernel) KillOne(s string) error {
	return k.kclnt.Kill(s)
}

func (k *Kernel) GetIP() string {
	return k.ip
}

func (k *Kernel) GetClnt() (*fslib.FsLib, *procclnt.ProcClnt) {
	return k.FsLib, k.ProcClnt
}

func (k *Kernel) MkClnt(name string, namedAddr []string) (*fslib.FsLib, *procclnt.ProcClnt, error) {
	fsl, err := fslib.MakeFsLibAddr(name, k.ip, namedAddr)
	if err != nil {
		return nil, nil, err
	}
	pclnt := procclnt.MakeProcClntInit(proc.GenPid(), fsl, name, namedAddr)
	return fsl, pclnt, nil
}

func (k *Kernel) Shutdown() error {
	k.kclnt.Shutdown()
	ctx := context.Background()
	db.DPrintf(db.CONTAINER, "containerwait for %v\n", k.container)
	statusCh, errCh := k.cli.ContainerWait(ctx, k.container, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		db.DPrintf(db.CONTAINER, "ContainerWait err %v\n", err)
		return err
	case st := <-statusCh:
		db.DPrintf(db.CONTAINER, "container %s done status %v\n", k.container, st)
	}
	out, err := k.cli.ContainerLogs(ctx, k.container, types.ContainerLogsOptions{ShowStderr: true, ShowStdout: true})
	if err != nil {
		panic(err)
	}
	stdcopy.StdCopy(os.Stdout, os.Stderr, out)
	removeOptions := types.ContainerRemoveOptions{
		RemoveVolumes: true,
		Force:         true,
	}
	if err := k.cli.ContainerRemove(ctx, k.container, removeOptions); err != nil {
		db.DPrintf(db.CONTAINER, "ContainerRemove %v err %v\n", k.container, err)
		return err
	}
	return nil
}

// XXX move into container package
func startContainer(yml string, nameds []string) (*Kernel, error) {
	ctx := context.Background()
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}
	env := makeEnv()
	fmt.Printf("start container %v %v %v\n", yml, nameds, env)
	resp, err := cli.ContainerCreate(ctx, &container.Config{
		Image: image,
		Cmd:   []string{"bin/linux/bootkernel", yml, fslib.NamedAddrsToString(nameds)},
		//AttachStdout: true,
		// AttachStderr: true,
		Tty: false,
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
