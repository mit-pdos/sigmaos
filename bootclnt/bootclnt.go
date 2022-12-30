package bootclnt

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"syscall"

	"sigmaos/container"
	db "sigmaos/debug"
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
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
	ip     string
}

func BootKernel(realmid string, contain bool, yml string) (*Kernel, error) {
	cmd := exec.Command("boot", []string{realmid, realmid + "/" + yml}...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	cmd.Stderr = os.Stderr
	cmd.Env = container.MakeEnv()

	if contain {
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
		if err := container.RunKernelContainer(cmd); err != nil {
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

	db.DPrintf(db.BOOTCLNT, "Wait for kernel to be booted\n")
	// wait for kernel to be booted
	s := ""
	ip := ""
	if _, err := fmt.Fscanf(stdout, "%s %s", &s, &ip); err != nil {
		db.DPrintf(db.BOOTCLNT, "Fscanf err %v %s\n", err, s)
		return nil, err
	}
	if s != RUNNING {
		db.DFatalf("oops: kernel is printing to stdout %s\n", s)
	}
	db.DPrintf(db.BOOTCLNT, "Kernel is running: %s at %s\n", s, ip)
	return &Kernel{cmd, stdin, stdout, ip}, nil
}

func (k *Kernel) Ip() string {
	return k.ip
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
	container.DelScnet(k.cmd.Process.Pid)
	return nil
}
