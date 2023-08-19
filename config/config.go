package config

import (
	"encoding/json"
	"os"
	"runtime/debug"
	"strconv"

	db "sigmaos/debug"
	"sigmaos/proc"
	sp "sigmaos/sigmap"
)

const (
	SIGMACONFIG = "SIGMACONFIG"
	SIGMADEBUG  = "SIGMADEBUG"
	SIGMAPERF   = "SIGMAPERF"
)

type SigmaConfig struct {
	PID        sp.Tpid   `json:pid,omitempty`
	Realm      sp.Trealm `json:realm,omitempty`
	Uname      sp.Tuname `json:uname,omitempty`
	KernelID   string    `json:kernelid,omitempty`
	UprocdPID  sp.Tpid   `json:uprocdpid,omitempty`
	Net        string    `json:net,omitempty`
	Privileged bool      `json:privileged,omitempty` // XXX phase out?
	Program    string    `json:program,omitempty`
	ProcDir    string    `json:procdir,omitempty`   // XXX phase out?
	ParentDir  string    `json:parentdir,omitempty` // XXX phase out?
	Perf       string    `json:perf,omitempty`
	Debug      string    `json:debug,omitempty`
	EtcdIP     string    `json:etcdip,omitempty`
	LocalIP    string    `json:localip,omitempty`
	BuildTag   string    `json:buildtag,omitempty`
	// For testing purposes
	Crash     bool `json:crash,omitempty`
	Partition bool `json:partition,omitempty`
}

// XXX will serve as a guide for necessary constructors
//func MakeFsLibAddrNet(uname sp.Tuname, realm sp.Trealm, lip string, addrs sp.Taddrs, clntnet string) (*FsLib, error) {
//func MakeFsLibAddr(uname sp.Tuname, realm sp.Trealm, lip string, addrs sp.Taddrs) (*FsLib, error) {
//func MakeFsLib(uname sp.Tuname) (*FsLib, error) {
//func MkSigmaClntRealmFsLib(rootrealm *fslib.FsLib, uname sp.Tuname, rid sp.Trealm) (*SigmaClnt, error) {
//func MkSigmaClntRootInit(uname sp.Tuname, ip string, namedAddr sp.Taddrs) (*SigmaClnt, error) {

func NewSigmaConfig() *SigmaConfig {
	// Load Perf & Debug from the environment for convenience.
	return &SigmaConfig{
		Perf:  os.Getenv(SIGMAPERF),
		Debug: os.Getenv(SIGMADEBUG),
	}
}

func NewBootSigmaConfig(uname sp.Tuname, etcdIP string) *SigmaConfig {
	sc := NewSigmaConfig()
	sc.EtcdIP = etcdIP
	sc.Uname = uname
	sc.Realm = sp.ROOTREALM
	return sc
}

func NewTestSigmaConfig(realm sp.Trealm, etcdIP, buildTag string) *SigmaConfig {
	sc := NewSigmaConfig()
	sc.Uname = "test"
	sc.Realm = realm
	sc.EtcdIP = etcdIP
	sc.BuildTag = buildTag
	return sc
}

// Create a new sigma config which is a derivative of an existing sigma config.
func NewAddedSigmaConfig(sc *SigmaConfig, idx int) *SigmaConfig {
	sc2 := NewSigmaConfig()
	*sc2 = *sc
	sc2.Uname = sp.Tuname(string(sc2.Uname) + "-" + strconv.Itoa(idx))
	return sc2
}

func NewChildSigmaConfig(pcfg *SigmaConfig, p *proc.Proc) *SigmaConfig {
	sc2 := NewSigmaConfig()
	*sc2 = *pcfg
	sc2.Uname = sp.Tuname(proc.GetPid())
	// TODO: anything else?
	return sc2
}

func (sc *SigmaConfig) Marshal() string {
	b, err := json.Marshal(sc)
	if err != nil {
		db.DFatalf("Error marshal sigmaconfig: %v")
	}
	return string(b)
}

// XXX When should I not get the config?
func GetSigmaConfig() *SigmaConfig {
	sc := NewSigmaConfig()
	scstr := os.Getenv(SIGMACONFIG)
	if scstr == "" {
		stack := debug.Stack()
		db.DFatalf("%s\nError: No Sigma Config", stack)
	}
	json.Unmarshal([]byte(scstr), sc)
	return sc
}
