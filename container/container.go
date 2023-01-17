package container

import (
	"fmt"
	"os"

	db "sigmaos/debug"
)

//
// exec-container enters here
//

const (
	PROC = "PROC"
)

func ExecContainer() error {
	db.DPrintf(db.CONTAINER, "execContainer: %v\n", os.Args)

	var r error
	switch os.Args[1] {
	case PROC:
		r = execPContainer()
	default:
		r = fmt.Errorf("ExecContainer: unknown container type: %s", os.Args[1])
	}
	return r
}
