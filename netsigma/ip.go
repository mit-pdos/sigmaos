package netsigma

import (
	"fmt"
	"net"
	"runtime/debug"
	"strings"

	db "sigmaos/debug"
	sp "sigmaos/sigmap"
)

// Rearrange addrs so that first addr is in the realm as clnt.
func Rearrange(clntnet string, addrs sp.Taddrs) sp.Taddrs {
	if len(addrs) == 1 {
		return addrs
	}
	raddrs := make(sp.Taddrs, len(addrs))
	for i := 0; i < len(addrs); i++ {
		raddrs[i] = addrs[i]
	}
	p := -1
	l := -1
	for i, a := range raddrs {
		if a.Net == clntnet {
			l = i
			break
		}
		if a.Net == sp.ROOTREALM.String() && p < 0 {
			p = i
		}
	}
	if l >= 0 {
		swap(raddrs, l)
	} else if p >= 0 {
		swap(raddrs, p)
	}
	return raddrs
}

func swap(addrs sp.Taddrs, i int) sp.Taddrs {
	v := addrs[0]
	addrs[0] = addrs[i]
	addrs[i] = v
	return addrs
}

func QualifyAddr(addr string) (string, error) {
	return QualifyAddrLocalIP("", addr)
}

func QualifyAddrLocalIP(lip string, addr string) (string, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		db.DFatalf("Err split host port %v: %v", addr, err)
		return "", err
	}
	if host == "::" {
		if lip == "" {
			ip, err := LocalIP()
			if err != nil {
				db.DFatalf("LocalIP \"%v\" %v", addr, err)
				return "", err
			}
			lip = ip
		}
		addr = net.JoinHostPort(lip, port)
	}
	return addr, nil
}

// XXX deduplicate with localIP
func LocalInterface() (string, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return "", err
	}
	for _, i := range ifaces {
		addrs, err := i.Addrs()
		if err != nil {
			return "", err
		}
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip.IsLoopback() {
				continue
			}
			if ip.To4() == nil {
				continue
			}
			return i.Name, nil
		}
	}
	return "", fmt.Errorf("localInterface: not found")
}

func localIPs() ([]net.IP, error) {
	var ips []net.IP
	ifaces, err := net.Interfaces()
	if err != nil {
		db.DFatalf("Err Get net interfaces %v: %v\n%s", ifaces, err, debug.Stack())
		return nil, err
	}
	for _, i := range ifaces {
		addrs, err := i.Addrs()
		if err != nil {
			return nil, err
		}
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip.IsLoopback() {
				continue
			}
			if ip.To4() == nil {
				continue
			}
			ips = append(ips, ip)
		}
	}
	return ips, nil
}

// XXX should find what outgoing ip is
func LocalIP() (string, error) {
	ips, err := localIPs()
	if err != nil {
		return "", err
	}

	// if we have a local ip in 10.10.x.x (for Cloudlab), prioritize that first
	for _, i := range ips {
		if strings.HasPrefix(i.String(), "10.10.") {
			return i.String(), nil
		}
		if !strings.HasPrefix(i.String(), "127.") {
			return i.String(), nil
		}
	}

	if len(ips) == 0 {
		return "", fmt.Errorf("LocalIP: no IP")
	}

	return ips[len(ips)-1].String(), nil
}
