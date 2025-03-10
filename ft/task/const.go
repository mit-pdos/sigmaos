package task

import (
	"sigmaos/namesrv/fsetcd"
	"time"
)

const (
	// server has to wait for any previous leaders to have their lease expire in etcd
	SRV_RESTART_TIMEOUT         = fsetcd.LeaseTTL * time.Second
)

const (
	MGR_RESTART_TIMEOUT         = SRV_RESTART_TIMEOUT + 500 * time.Millisecond
	MGR_PING_TIMEOUT            = 500 * time.Millisecond
	MGR_NUM_FAILS_UNTIL_RESTART = 3
)

const (
	// total wait time should be longer than server restart timeout
	CLNT_NUM_RETRIES            = 20
	CLNT_RETRY_TIMEOUT          = 500 * time.Millisecond
)