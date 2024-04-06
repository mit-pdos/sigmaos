package proc

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path"
	"runtime/debug"
	"strings"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	sp "sigmaos/sigmap"
)

// Environment variables for procs (SHOULD NOT BE ADDED TO)
const (
	SIGMASTRACE   = "SIGMASTRACE"
	SIGMADEBUGPID = "SIGMADEBUGPID"
	SIGMAPERF     = "SIGMAPERF"
	SIGMADEBUG    = "SIGMADEBUG"
	SIGMACONFIG   = "SIGMACONFIG"
)

type ProcEnv struct {
	*ProcEnvProto
}

func GetProcEnv() *ProcEnv {
	pestr := os.Getenv(SIGMACONFIG)
	if pestr == "" {
		stack := debug.Stack()
		log.Fatalf("FATAL %v: No ProcEnv\n%s", GetSigmaDebugPid(), stack)
	}
	return Unmarshal(pestr)
}

func SetSigmaDebugPid(pid string) {
	os.Setenv(SIGMADEBUGPID, pid)
}

func GetSigmaDebugPid() string {
	return os.Getenv(SIGMADEBUGPID)
}

func GetSigmaPerf() string {
	return os.Getenv(SIGMAPERF)
}

func GetSigmaDebug() string {
	return os.Getenv(SIGMADEBUG)
}

func GetLabelsEnv(envvar string) map[string]bool {
	s := os.Getenv(envvar)
	return GetLabels(s)
}

func GetLabels(s string) map[string]bool {
	m := make(map[string]bool)
	if s == "" {
		return m
	}
	labels := strings.Split(s, ";")
	for _, l := range labels {
		if l != "" {
			m[l] = true
		}
	}
	return m
}

func NewProcEnv(program string, pid sp.Tpid, realm sp.Trealm, principal *sp.Tprincipal, procDir string, parentDir string, priv, overlays, useSigmaclntd bool, useNetProxy bool) *ProcEnv {
	// Load Perf & Debug from the environment for convenience.
	return &ProcEnv{
		ProcEnvProto: &ProcEnvProto{
			PidStr:              string(pid),
			RealmStr:            string(realm),
			Principal:           principal,
			ProcDir:             procDir,
			ParentDir:           parentDir,
			Program:             program,
			InnerContainerIPStr: sp.NOT_SET,
			OuterContainerIPStr: sp.NOT_SET,
			KernelID:            sp.NOT_SET,
			BuildTag:            sp.NOT_SET,
			Net:                 sp.NOT_SET,
			Perf:                os.Getenv(SIGMAPERF),
			Strace:              os.Getenv(SIGMASTRACE),
			Debug:               os.Getenv(SIGMADEBUG),
			UprocdPIDStr:        sp.NOT_SET,
			Privileged:          priv,
			Overlays:            overlays,
			UseSigmaclntd:       useSigmaclntd,
			UseNetProxy:         useNetProxy,
			Claims: &ProcClaimsProto{
				PrincipalIDStr: principal.GetID().String(),
				RealmStr:       sp.NOT_SET,
				AllowedPaths:   nil, // By default, will be set to the parent's AllowedPaths unless otherwise specified
				Secrets:        nil, // By default, will be set to the parent's Secrets unless otherwise specified
			},
		},
	}
}

func NewProcEnvUnset(priv, overlays bool) *ProcEnv {
	// Load Perf & Debug from the environment for convenience.
	return NewProcEnv(sp.NOT_SET, sp.Tpid(sp.NOT_SET), sp.Trealm(sp.NOT_SET), sp.NoPrincipal(), sp.NOT_SET, sp.NOT_SET, priv, overlays, false, false)
}

func NewProcEnvFromProto(p *ProcEnvProto) *ProcEnv {
	return &ProcEnv{p}
}

func NewBootProcEnv(principal *sp.Tprincipal, secrets map[string]*ProcSecretProto, etcdMnts map[string]*sp.TmountProto, innerIP sp.Tip, outerIP sp.Tip, buildTag string, overlays bool) *ProcEnv {
	pe := NewProcEnvUnset(true, overlays)
	pe.SetPrincipal(principal)
	pe.SetSecrets(secrets)
	// Allow all paths for boot env
	pe.SetAllowedPaths(sp.ALL_PATHS)
	pe.Program = "kernel"
	pe.SetPID(sp.Tpid(principal.GetID().String()))
	pe.EtcdMounts = etcdMnts
	pe.InnerContainerIPStr = innerIP.String()
	pe.OuterContainerIPStr = outerIP.String()
	pe.BuildTag = buildTag
	pe.SetRealm(sp.ROOTREALM, overlays)
	pe.ProcDir = path.Join(sp.KPIDS, pe.GetPID().String())
	pe.Privileged = true
	pe.HowInt = int32(BOOT)
	return pe
}

func NewTestProcEnv(realm sp.Trealm, secrets map[string]*ProcSecretProto, etcdMnts map[string]*sp.TmountProto, innerIP sp.Tip, outerIP sp.Tip, buildTag string, overlays, useSigmaclntd bool, useNetProxy bool) *ProcEnv {
	pe := NewProcEnvUnset(true, overlays)
	pe.SetPrincipal(sp.NewPrincipal(sp.TprincipalID("test"), realm, sp.NoToken()))
	pe.SetSecrets(secrets)
	// Allow all paths for boot env
	pe.SetAllowedPaths(sp.ALL_PATHS)
	pe.SetPID(sp.GenPid("test"))
	pe.SetRealm(realm, overlays)
	pe.EtcdMounts = etcdMnts
	pe.InnerContainerIPStr = innerIP.String()
	pe.OuterContainerIPStr = outerIP.String()
	pe.BuildTag = buildTag
	pe.Program = "test"
	pe.ProcDir = path.Join(sp.KPIDS, pe.GetPID().String())
	pe.HowInt = int32(TEST)
	pe.UseSigmaclntd = useSigmaclntd
	pe.UseNetProxy = useNetProxy
	return pe
}

// Create a new sigma config which is a derivative of an existing sigma config.
func NewAddedProcEnv(pe *ProcEnv) *ProcEnv {
	pe2 := NewProcEnvUnset(pe.Privileged, false)
	*(pe2.ProcEnvProto) = *(pe.ProcEnvProto)
	pe2.SetPrincipal(sp.NewPrincipal(pe.GetPrincipal().GetID(), pe.GetRealm(), pe.GetPrincipal().GetToken()))
	// Make a deep copy of the proc claims
	pe2.Claims = &ProcClaimsProto{
		PrincipalIDStr: pe2.GetPrincipal().GetID().String(),
		RealmStr:       pe2.GetPrincipal().GetRealm().String(),
		AllowedPaths:   make([]string, len(pe.Claims.GetAllowedPaths())),
		Secrets:        make(map[string]*ProcSecretProto),
	}
	// Deep copy allowed paths
	copy(pe2.Claims.AllowedPaths, pe.Claims.GetAllowedPaths())
	// Deep copy secrets
	for k, v := range pe.Claims.GetSecrets() {
		pe2.Claims.Secrets[k] = &ProcSecretProto{
			ID:  v.ID,
			Key: v.Key,
		}
	}
	return pe2
}

func NewDifferentRealmProcEnv(pe *ProcEnv, realm sp.Trealm) *ProcEnv {
	pe2 := NewProcEnvUnset(pe.Privileged, pe.Overlays)
	*(pe2.ProcEnvProto) = *(pe.ProcEnvProto)
	pe2.SetPrincipal(sp.NewPrincipal(
		sp.TprincipalID(pe.GetPrincipal().GetID().String()+"-realm-"+realm.String()),
		realm,
		sp.NoToken(),
	))
	// Make a deep copy of the proc claims
	pe2.Claims = &ProcClaimsProto{
		PrincipalIDStr: pe2.GetPrincipal().GetID().String(),
		RealmStr:       realm.String(),
		AllowedPaths:   make([]string, len(pe.Claims.GetAllowedPaths())),
		Secrets:        make(map[string]*ProcSecretProto),
	}
	pe2.SetRealm(realm, pe.Overlays)
	// Deep copy allowed paths
	copy(pe2.Claims.AllowedPaths, pe.Claims.GetAllowedPaths())
	// Deep copy secrets
	for k, v := range pe.Claims.GetSecrets() {
		pe2.Claims.Secrets[k] = &ProcSecretProto{
			ID:  v.ID,
			Key: v.Key,
		}
	}
	return pe2
}

func (pe *ProcEnvProto) GetPID() sp.Tpid {
	return sp.Tpid(pe.PidStr)
}

func (pe *ProcEnvProto) SetSecrets(secrets map[string]*ProcSecretProto) {
	pe.Claims.Secrets = secrets
}

func (pe *ProcEnvProto) SetToken(token *sp.Ttoken) {
	pe.Principal.SetToken(token)
}

func (pe *ProcEnvProto) SetAllowedPaths(paths []string) {
	pe.Claims.AllowedPaths = paths
}

func (pe *ProcEnvProto) SetPID(pid sp.Tpid) {
	pe.PidStr = string(pid)
}

func (pe *ProcEnvProto) SetInnerContainerIP(ip sp.Tip) {
	pe.InnerContainerIPStr = ip.String()
}

func (pe *ProcEnvProto) GetSecrets() map[string]*ProcSecretProto {
	secrets := make(map[string]*ProcSecretProto)
	// Deep copy secrets
	for k, v := range pe.Claims.GetSecrets() {
		secrets[k] = &ProcSecretProto{
			ID:  v.ID,
			Key: v.Key,
		}
	}
	return secrets
}

func (pe *ProcEnvProto) GetInnerContainerIP() sp.Tip {
	return sp.Tip(pe.InnerContainerIPStr)
}

func (pe *ProcEnvProto) SetOuterContainerIP(ip sp.Tip) {
	pe.OuterContainerIPStr = ip.String()
}

func (pe *ProcEnvProto) GetOuterContainerIP() sp.Tip {
	return sp.Tip(pe.OuterContainerIPStr)
}

func (pe *ProcEnvProto) GetRealm() sp.Trealm {
	return sp.Trealm(pe.RealmStr)
}

func (pe *ProcEnvProto) SetRealm(realm sp.Trealm, overlays bool) {
	pe.RealmStr = realm.String()
	pe.Principal.RealmStr = realm.String()
	pe.Claims.RealmStr = realm.String()
	// Changing the realm changes the overlay network name. Therefore, set the
	// overlay network for the new realm.
	pe.Net = sp.ROOTREALM.String()
	if overlays {
		pe.Net = "sigmanet-" + realm.String()
		if realm == sp.ROOTREALM {
			pe.Net = "sigmanet-testuser"
		}
	}
}

func (pe *ProcEnvProto) SetPrincipal(principal *sp.Tprincipal) {
	pe.Principal = principal
	pe.Claims.PrincipalIDStr = principal.GetID().String()
}

func (pe *ProcEnvProto) SetUprocdPID(pid sp.Tpid) {
	pe.UprocdPIDStr = string(pid)
}

func (pe *ProcEnvProto) GetUprocdPID() sp.Tpid {
	return sp.Tpid(pe.UprocdPIDStr)
}

func (pe *ProcEnv) GetProto() *ProcEnvProto {
	return pe.ProcEnvProto
}

func (pe *ProcEnvProto) SetNetFail(nf int64) {
	pe.NetFail = nf
}

func (pe *ProcEnvProto) SetCrash(nf int64) {
	pe.Crash = nf
}

func (pe *ProcEnvProto) SetPartition(nf int64) {
	pe.Partition = nf
}

func (pe *ProcEnvProto) SetHow(how Thow) {
	pe.HowInt = int32(how)
}

func (pe *ProcEnvProto) GetHow() Thow {
	return Thow(pe.HowInt)
}

func (pe *ProcEnvProto) SetSpawnTime(t time.Time) {
	pe.SpawnTimePB = timestamppb.New(t)
}

func (pe *ProcEnvProto) GetSpawnTime() time.Time {
	return pe.SpawnTimePB.AsTime()
}

func (pe *ProcEnv) GetScheddMount() (*sp.Tmount, bool) {
	mp := pe.ProcEnvProto.GetScheddMountProto()
	if mp == nil {
		return &sp.Tmount{}, false
	}
	return sp.NewMountFromProto(mp), true
}

func (pe *ProcEnv) GetNamedMount() (*sp.Tmount, bool) {
	mp := pe.ProcEnvProto.GetNamedMountProto()
	if mp == nil {
		return &sp.Tmount{}, false
	}
	return sp.NewMountFromProto(mp), true
}

func (pe *ProcEnv) Marshal() string {
	b, err := json.Marshal(pe)
	if err != nil {
		log.Fatalf("FATAL %v: Error marshal ProcEnv: %v", GetSigmaDebugPid(), err)
	}
	return string(b)
}

func Unmarshal(pestr string) *ProcEnv {
	pe := &ProcEnv{}
	err := json.Unmarshal([]byte(pestr), pe)
	if err != nil {
		log.Fatalf("FATAL %v: Error unmarshal ProcEnv: %v", GetSigmaDebugPid(), err)
	}
	return pe
}

// TODO: cleanup
func (pe *ProcEnv) String() string {
	return fmt.Sprintf("&{ Program: %v Pid:%v Realm:%v Principal:%v KernelID:%v UprocdPID:%v Net:%v ProcDir:%v ParentDir:%v How:%v Perf:%v Debug:%v EtcdMnt:%v InnerIP:%v OuterIP:%v BuildTag:%v Privileged:%v Overlays:%v Crash:%v Partition:%v NetFail:%v UseSigmaclntd:%v UseNetProxy:%v Claims:%v }", pe.Program, pe.GetPID(), pe.GetRealm(), pe.GetPrincipal().String(), pe.KernelID, pe.UprocdPIDStr, pe.Net, pe.ProcDir, pe.ParentDir, Thow(pe.HowInt), pe.Perf, pe.Debug, pe.GetEtcdMounts(), pe.InnerContainerIPStr, pe.OuterContainerIPStr, pe.BuildTag, pe.Privileged, pe.Overlays, pe.Crash, pe.Partition, pe.NetFail, pe.UseSigmaclntd, pe.UseNetProxy, pe.Claims)
}
