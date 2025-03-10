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
	// time between manager respawning a server and starting to ping it again to avoid pinging while server is restarting
	MGR_RESTART_TIMEOUT         = SRV_RESTART_TIMEOUT + 500 * time.Millisecond
	MGR_PING_TIMEOUT            = 500 * time.Millisecond
	MGR_NUM_FAILS_UNTIL_RESTART = 2
)

const (
	// number of retries before giving up on a RPC call to server
	CLNT_NUM_RETRIES            = 20
	CLNT_RETRY_TIMEOUT          = 500 * time.Millisecond

	// give up much faster while pinging server to allow server to respawn while client is still retrying
	CLNT_NUM_RETRIES_PING       = 5
)