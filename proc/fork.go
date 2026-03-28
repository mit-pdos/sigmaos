package proc

import (
	"crypto/sha256"
	"encoding/hex"
	"maps"
	"time"

	"google.golang.org/protobuf/proto"
)

// ForkConfig specifies how to create a Zygote if no matching Zygote exists.
//
// For now, this only captures the initialization procedure. The runtime state
// (e.g., live Zygote discovery, child args, supervisor socket protocol) is
// handled by the scheduler/procd.
type ForkConfig struct {
	// ZygoteProc describes the proc that should be started and then used as the
	// fork parent (the "Zygote") when there is no matching warm Zygote.
	ZygoteProc *Proc

	// KeepAlive is the maximum idle time to keep a Zygote alive (i.e., when it has
	// no living children). A value of 0 or less means that the Zygote should be
	// terminated immediately after its last child exits.
	KeepAlive time.Duration
}

// NewForkProc creates a proc that should be executed by forking a matching
// Zygote (as specified by cfg). The forked child's argument vector is provided
// separately from the Zygote's own args.
func NewForkProc(cfg ForkConfig, childArgs []string) *Proc {
	if cfg.ZygoteProc == nil {
		return nil
	}

	// Clone the Zygote proc definition but assign a fresh pid.
	clone := NewProc(cfg.ZygoteProc.GetProgram(), append([]string{}, cfg.ZygoteProc.Args...))
	clone.GetProcEnv().UseSPProxy = cfg.ZygoteProc.GetProcEnv().UseSPProxy
	clone.GetProcEnv().UseSPProxyProcClnt = cfg.ZygoteProc.GetProcEnv().UseSPProxyProcClnt
	clone.Env = maps.Clone(cfg.ZygoteProc.Env)

	// Compute a stable zygote key from the initialization procedure.
	keyMsg := &ForkProcProto{
		ZygoteProgram: cfg.ZygoteProc.GetProgram(),
		ZygoteArgs:    append([]string{}, cfg.ZygoteProc.Args...),
		ZygoteEnv:     clone.Env,
	}
	b, _ := (proto.MarshalOptions{Deterministic: true}).Marshal(keyMsg)
	h := sha256.Sum256(b)
	zygoteKey := hex.EncodeToString(h[:])

	clone.ForkProc = &ForkProcProto{
		ZygoteKey:     zygoteKey,
		ZygoteProgram: cfg.ZygoteProc.GetProgram(),
		ZygoteArgs:    append([]string{}, cfg.ZygoteProc.Args...),
		ZygoteEnv:     clone.Env,
		KeepAliveNs:   uint64(cfg.KeepAlive),
		ChildArgs:     append([]string{}, childArgs...),
	}
	return clone
}
