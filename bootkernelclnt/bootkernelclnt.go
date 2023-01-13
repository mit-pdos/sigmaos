package bootkernelclnt

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"syscall"
	// "time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	// "github.com/docker/docker/pkg/stdcopy"

	sc "sigmaos/container"
	db "sigmaos/debug"
	"sigmaos/frame"
	"sigmaos/kernel"
	"sigmaos/yaml"
)

const (
	RUNNING  = "running"
	SHUTDOWN = "shutdown"

	HOME = "/home/sigmaos"
)

//
// Library to start a kernel boot process.  Because this library boots
// the first named, it uses a pipe to talk to the boot process; we
// cannot use named to connect to it.
//

type Kernel struct {
	cmd         *exec.Cmd
	stdin       io.WriteCloser
	stdout      io.ReadCloser
	ip          string
	realmid     string
	cli         *client.Client
	containerid string
}

func BootKernel1(image string, yml string) (*Kernel, error) {
	ctx := context.Background()
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}
	log.Printf("create container from image %v\n", image)
	resp, err := cli.ContainerCreate(ctx, &container.Config{
		Image: image,
		Cmd:   []string{"bin/linux/bootkernel", "bootkernelclnt/boot.yml"},
		//AttachStdout: true,
		// AttachStderr: true,
		// Tty:          false,
	}, nil, nil, nil, "")
	if err := cli.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
		return nil, err
	}
	json, err1 := cli.ContainerInspect(ctx, resp.ID)
	if err1 != nil {
		return nil, err
	}
	ip := json.NetworkSettings.IPAddress
	log.Printf("json %v\n", ip)
	return &Kernel{nil, nil, nil, ip, "", cli, resp.ID}, nil
}

func BootKernel(realmid string, contain bool, yml string) (*Kernel, error) {
	cmd := exec.Command("bootkernel")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	cmd.Stderr = os.Stderr
	cmd.Env = sc.MakeEnv()

	if contain {
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
		if err := sc.RunKernelContainer(cmd, realmid); err != nil {
			return nil, err
		}
	} else {
		// Create a process group ID to kill all children if necessary.
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
		if err := cmd.Start(); err != nil {
			db.DPrintf(db.BOOTCLNT, "BootKernel: Start err %v\n", err)
			return nil, err
		}
	}

	db.DPrintf(db.BOOTCLNT, "Yaml %v\n", yml)
	param := kernel.Param{}
	err = yaml.ReadYaml(yml, &param)
	if err != nil {
		return nil, err
	}

	db.DPrintf(db.BOOTCLNT, "Yaml %v\n", param)
	param.Realm = realmid
	ip, err := sc.LocalIP()
	if err != nil {
		return nil, err
	}
	param.Hostip = ip
	b, err := yaml.Marshal(param)
	if err != nil {
		return nil, err
	}

	db.DPrintf(db.BOOTCLNT, "Yaml:%d\n", len(b))

	if err := frame.WriteFrame(stdin, b); err != nil {
		return nil, err
	}

	db.DPrintf(db.BOOTCLNT, "Wait for kernel to be booted\n")
	// wait for kernel to be booted
	s := ""
	if _, err := fmt.Fscanf(stdout, "%s %s", &s, &ip); err != nil {
		db.DPrintf(db.BOOTCLNT, "Fscanf err %v %s\n", err, s)
		return nil, err
	}
	if s != RUNNING {
		db.DFatalf("oops: kernel is printing to stdout %s\n", s)
	}
	db.DPrintf(db.BOOTCLNT, "Kernel is running: %s at %s\n", s, ip)
	return &Kernel{cmd, stdin, stdout, ip, realmid, nil, ""}, nil
}

func (k *Kernel) Ip() string {
	return k.ip
}

func (k *Kernel) Shutdown1() error {
	ctx := context.Background()
	return k.cli.ContainerKill(ctx, k.containerid, "SIGTERM")
}

func (k *Kernel) Shutdown() error {
	defer k.stdout.Close()
	if _, err := io.WriteString(k.stdin, SHUTDOWN+"\n"); err != nil {
		return err
	}
	defer k.stdin.Close()
	db.DPrintf(db.BOOTCLNT, "Wait for kernel to shutdown\n")
	if err := k.cmd.Wait(); err != nil {
		return err
	}
	if err := sc.DelScnet(k.cmd.Process.Pid, k.realmid); err != nil {
		return err
	}
	return nil
}
