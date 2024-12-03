package ip

import (
	"fmt"
	"net"
	"runtime/debug"
	"strings"

	db "sigmaos/debug"
	sp "sigmaos/sigmap"
)

func QualifyAddrLocalIP(lip sp.Tip, addrstr string) (sp.Tip, sp.Tport, error) {
	h, pstr, err := net.SplitHostPort(addrstr)
	if err != nil {
		db.DPrintf(db.ERROR, "Err split host port %v: %v", addrstr, err)
		return sp.NO_IP, sp.NO_PORT, err
	}
	p, err := sp.ParsePort(pstr)
	if err != nil {
		db.DPrintf(db.ERROR, "Err split host port %v: %v", addrstr, err)
		return sp.NO_IP, sp.NO_PORT, err
	}
	var host sp.Tip = lip
	var port sp.Tport = p
	if h == "::" {
		if lip == "" {
			ip, err := LocalIP()
			if err != nil {
				db.DPrintf(db.ERROR, "LocalIP \"%v\" %v", addrstr, err)
				return sp.NO_IP, sp.NO_PORT, err
			}
			host = ip
		}
	}
	return host, port, nil
}

func localIPs() ([]net.IP, error) {
	var ips []net.IP
	ifaces, err := net.Interfaces()
	if err != nil {
		db.DPrintf(db.ERROR, "Err Get net interfaces %v: %v\n%s", ifaces, err, debug.Stack())
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
func LocalIP() (sp.Tip, error) {
	ips, err := localIPs()
	if err != nil {
		return "", err
	}

	// if we have a local ip in 10.10.x.x (for Cloudlab), prioritize that first
	for _, i := range ips {
		if strings.HasPrefix(i.String(), "10.10.") {
			return sp.Tip(i.String()), nil
		}
	}
	// if we have a local ip in 10.0.x.x (for Docker), prioritize that next
	for _, i := range ips {
		if strings.HasPrefix(i.String(), "10.0.") {
			return sp.Tip(i.String()), nil
		}
	}
	// XXX Should do this in a more principled way
	// Next, prioritize non-localhost IPs
	for _, i := range ips {
		if !strings.HasPrefix(i.String(), "127.") {
			return sp.Tip(i.String()), nil
		}
	}

	if len(ips) == 0 {
		return "", fmt.Errorf("LocalIP: no IP")
	}

	return sp.Tip(ips[len(ips)-1].String()), nil
}
