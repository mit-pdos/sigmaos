package system

import (
	"os/exec"
	db "sigmaos/debug"
)

const (
	START = "../start.sh"
)

func Start() (string, error) {
	db.DPrintf(db.BOOT, "Boot %v\n", START)
	out, err := exec.Command(START, []string{}...).Output()
	if err != nil {
		db.DPrintf(db.BOOT, "Boot failed %s err %v\n", string(out), err)
		return "", err
	}
	return string(out), nil
}
