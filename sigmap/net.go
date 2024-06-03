package sigmap

import (
	"encoding/json"
	"log"
	"net"
	"strconv"
)

func (iptype Tiptype) String() string {
	switch iptype {
	case INNER_CONTAINER_IP:
		return "innerIP"
	case OUTER_CONTAINER_IP:
		return "outerIP"
	default:
		log.Fatalf("Error unkown ip type: %v", uint32(iptype))
		return "unknown"
	}
}

func (p Tport) String() string {
	return strconv.FormatUint(uint64(p), 10)
}

func (p Tip) String() string {
	return string(p)
}

func ParsePort(ps string) (Tport, error) {
	pi, err := strconv.ParseUint(ps, 10, 32)
	return Tport(pi), err
}

func (a *Taddr) IPPort() string {
	return a.IPStr + ":" + a.GetPort().String()
}

func (a *Taddr) GetIP() Tip {
	return Tip(a.IPStr)
}

func (a *Taddr) GetPort() Tport {
	return Tport(a.PortInt)
}

func NewTaddrAnyPort(iptype Tiptype) *Taddr {
	return NewTaddrRealm(NO_IP, iptype, NO_PORT)
}

func NewTaddrFromString(address string, iptype Tiptype) (*Taddr, error) {
	h, po, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}
	port, err := strconv.Atoi(po)
	if err != nil {
		return nil, err
	}
	return NewTaddrRealm(Tip(h), iptype, Tport(port)), nil
}

func NewTaddr(ip Tip, iptype Tiptype, port Tport) *Taddr {
	return &Taddr{
		IPStr:     string(ip),
		IPTypeInt: uint32(iptype),
		PortInt:   uint32(port),
	}
}

func NewTaddrRealm(ip Tip, iptype Tiptype, port Tport) *Taddr {
	return &Taddr{
		IPStr:     string(ip),
		IPTypeInt: uint32(iptype),
		PortInt:   uint32(port),
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
		s += a.IPPort()
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
