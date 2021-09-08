package realm

import (
	"log"
	"os"
	"os/exec"
	"path"
	"syscall"
	"time"

	"ulambda/fslib"
	"ulambda/named"
)

const (
	SLEEP_MS = 1000
)

// Boot a named and set up the initfs
func BootNamed(bin string, addr string) (*exec.Cmd, error) {
	cmd, err := run(bin, "/bin/kernel/named", addr, []string{"0", addr})
	if err != nil {
		return nil, err
	}
	time.Sleep(SLEEP_MS * time.Millisecond)
	fsl := fslib.MakeFsLibAddr("kernel", addr)
	if err := named.MakeInitFs(fsl); err != nil {
		return nil, err
	}
	return cmd, nil
}

func ShutdownNamed(namedAddr string) {
	fsl := fslib.MakeFsLibAddr("kernel", namedAddr)
	// Shutdown named last
	err := fsl.Remove(named.NAMED + "/")
	if err != nil {
		// XXX sometimes we get EOF..
		if err.Error() == "EOF" {
			log.Printf("Remove %v shutdown %v\n", named.NAMED, err)
		} else {
			log.Fatalf("Remove %v shutdown %v\n", named.NAMED, err)
		}
	}
	time.Sleep(SLEEP_MS * time.Millisecond)
}

func run(bin string, name string, namedAddr string, args []string) (*exec.Cmd, error) {
	cmd := exec.Command(path.Join(bin, name), args...)
	// Create a process group ID to kill all children if necessary.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ())
	cmd.Env = append(cmd.Env, "NAMED="+namedAddr)
	return cmd, cmd.Start()
}
