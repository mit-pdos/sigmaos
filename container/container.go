package container

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"syscall"

	db "sigmaos/debug"
)

//
// exec-container enters here
//

const (
	KERNEL = "KERNEL"
	PROC   = "PROC"
)

func ExecContainer(rootfs string) error {
	log.Printf("ExecContainer %v %v\n", os.Args, os.Environ())

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

	if err := syscall.Chdir(os.Getenv("HOME")); err != nil {
		log.Printf("failed to chdir to /: %v", err)
		return err
	}

	pn, err := exec.LookPath(os.Args[1])
	if err != nil {
		return fmt.Errorf("LookPath: %v", err)
	}
	db.DPrintf(db.CONTAINER, "exec %s %v\n", pn, os.Args[1:])
	return syscall.Exec(pn, os.Args[1:], os.Environ())
}
