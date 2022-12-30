package container

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"syscall"

	db "sigmaos/debug"
	sp "sigmaos/sigmap"
)

//
// exec-container enters here
//

const (
	KERNEL = "KERNEL"
	PROC   = "PROC"
)

var envvar = []string{"SIGMADEBUG", "SIGMAPERF", "SIGMAROOTFS"}

func SIGMAROOTFS() (string, error) {
	fs := os.Getenv("SIGMAROOTFS")
	if fs == "" {
		return "", fmt.Errorf("%v: ExecContainer: SIGMAROOTFS isn't set; `run source env/init.sh`\n", os.Args[0])
	}
	return fs, nil
}

func MakeEnv() []string {
	env := []string{}
	for _, s := range envvar {
		if e := os.Getenv(s); e != "" {
			env = append(env, fmt.Sprintf("%s=%s", s, e))
		}
	}
	return env
}

func ExecContainer() error {
	log.Printf("ExecContainer Args %v Env %v\n", os.Args, os.Environ())

	rootfs, err := SIGMAROOTFS()
	if err != nil {
		return err
	}

	var r error
	switch os.Args[0] {
	case KERNEL:
		r = setupKContainer(rootfs)
	case PROC:
		r = setupPContainer()
	default:
		r = fmt.Errorf("ExecContainer: unknown container type: %s", os.Args[0])
	}
	if r != nil {
		return r
	}

	if err := syscall.Chdir(sp.SIGMAHOME); err != nil {
		log.Printf("Chdir %s err %v", sp.SIGMAHOME, err)
		return err
	}

	path := os.Getenv("PATH")
	p := sp.SIGMAHOME + "/bin/linux/:" + sp.SIGMAHOME + "/bin/kernel"
	os.Setenv("PATH", path+":"+p)

	db.DPrintf(db.CONTAINER, "env: %v\n", os.Environ())

	pn, err := exec.LookPath(os.Args[1])
	if err != nil {
		return fmt.Errorf("LookPath err %v", err)
	}
	db.DPrintf(db.CONTAINER, "exec %s %v\n", pn, os.Args[1:])
	return syscall.Exec(pn, os.Args[1:], os.Environ())
}
