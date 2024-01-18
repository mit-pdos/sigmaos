package sigmap

import (
	"encoding/json"
	"log"
	"strconv"
)

func (p Tport) String() string {
	return strconv.FormatUint(uint64(p), 10)
}

func (p Thost) String() string {
	return string(p)
}

func ParsePort(ps string) (Tport, error) {
	pi, err := strconv.ParseUint(ps, 10, 32)
	return Tport(pi), err
}

func (a *Taddr) HostPort() string {
	return a.HostStr + ":" + a.GetPort().String()
}

func (a *Taddr) GetHost() Thost {
	return Thost(a.HostStr)
}

func (a *Taddr) GetPort() Tport {
	return Tport(a.PortInt)
}

func NewTaddrAnyPort(netns string) *Taddr {
	return NewTaddrRealm(NO_HOST, NO_PORT, netns)
}

func NewTaddr(host Thost, port Tport) *Taddr {
	return &Taddr{
		HostStr: string(host),
		PortInt: uint32(port),
		NetNS:   ROOTREALM.String(),
	}
}

func NewTaddrRealm(host Thost, port Tport, netns string) *Taddr {
	return &Taddr{
		HostStr: string(host),
		PortInt: uint32(port),
		NetNS:   netns,
	}
}

func (a *Taddr) Marshal() string {
	b, err := json.Marshal(a)
	if err != nil {
		log.Fatalf("Can't marshal Taddr: %v", err)
	}
	return string(b)
}

func UnmarshalTaddr(a string) *Taddr {
	var addr Taddr
	err := json.Unmarshal([]byte(a), &addr)
	if err != nil {
		log.Fatalf("Can't unmarshal Taddr")
	}
	return &addr
}

//func NewTaddrs(addr []string) Taddrs {
//	addrs := make([]*Taddr, len(addr))
//	for i, a := range addr {
//		addrs[i] = NewTaddr(a)
//	}
//	return addrs
//}

// Ignores net
func (as Taddrs) String() string {
	s := ""
	for i, a := range as {
		s += a.HostPort()
		if i < len(as)-1 {
			s += ","
		}
	}
	return s
}

// Includes net. In the future should return a mnt, but we need to
// package it up in a way suitable to pass as argument or environment
// variable to a program.
func (as Taddrs) Taddrs2String() (string, error) {
	b, err := json.Marshal(as)
	return string(b), err
}

func String2Taddrs(as string) (Taddrs, error) {
	var addrs Taddrs
	err := json.Unmarshal([]byte(as), &addrs)
	return addrs, err
}
