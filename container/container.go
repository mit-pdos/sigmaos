package container

import (
	"fmt"
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

func ExecContainer() error {
	db.DPrintf(db.CONTAINER, "ExecContainer %v\n", os.Args)

	var r error
	switch os.Args[0] {
	case KERNEL:
		r = setupKContainer()
	case PROC:
		r = setupPContainer()
	default:
		r = fmt.Errorf("ExecContainer: unknown container type: %s", os.Args[0])
	}
	if r != nil {
		return r
	}
	pn, err := exec.LookPath(os.Args[1])
	if err != nil {
		return fmt.Errorf("LookPath: %v", err)
	}
	db.DPrintf(db.CONTAINER, "exec %s %v\n", pn, os.Args[2:])
	return syscall.Exec(pn, os.Args[2:], os.Environ())
}
