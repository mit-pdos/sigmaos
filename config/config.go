package config

import (
	"encoding/json"
	"os"
	"runtime/debug"

	db "sigmaos/debug"
	"sigmaos/perf"
	sp "sigmaos/sigmap"
)

const (
	SIGMACONFIG = "SIGMACONFIG"
)

type SigmaConfig struct {
	PID        sp.Tpid        `json:pid,omitempty`
	Realm      sp.Trealm      `json:realm,omitempty`
	Uname      sp.Tuname      `json:uname,omitempty`
	KernelID   string         `json:kernelid,omitempty`
	UprocdPID  sp.Tpid        `json:uprocdpid,omitempty`
	Net        string         `json:net,omitempty`
	Privileged bool           `json:privileged,omitempty` // XXX phase out?
	Program    string         `json:program,omitempty`
	ProcDir    string         `json:procdir,omitempty`   // XXX phase out?
	ParentDir  string         `json:parentdir,omitempty` // XXX phase out?
	Perf       perf.Tselector `json:perf,omitempty`
	Debug      db.Tselector   `json:debug,omitempty`
	EtcdAddr   string         `json:etcdaddr,omitempty`
	LocalIP    string         `json:localip,omitempty`
	BuildTag   string         `json:buildtag,omitempty`
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
	return &SigmaConfig{}
}

func NewTestSigmaConfig(realm sp.Trealm, etcdaddr string) *SigmaConfig {
	sc := NewSigmaConfig()
	sc.Realm = realm
	sc.EtcdAddr = etcdaddr
	return sc
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
