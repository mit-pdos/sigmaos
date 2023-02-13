package container

import (
	"fmt"
	"net"
	"strings"

	sp "sigmaos/sigmap"
)

func Rearrange(realm sp.Trealm, addrs sp.Taddrs) sp.Taddrs {
	ip, err := LocalIP()
	if err == nil {
		addrs = rearrange(addrs, ip)
	}
	return addrs
}

// Rearrange addrs so that first addr is on same network as ip. If
// none, prefer a non-10 address.
func rearrange(addrs sp.Taddrs, ip string) sp.Taddrs {
	if len(addrs) == 1 {
		return addrs
	}
	raddrs := make(sp.Taddrs, len(addrs))
	for i := 0; i < len(addrs); i++ {
		raddrs[i] = addrs[i]
	}
	local := false
	p := 0
	l := 0
	for i, a := range raddrs {
		h1, _, r := net.SplitHostPort(a.Addr)
		if r != nil {
			return raddrs
		}
		ip1, ipnet1, r := net.ParseCIDR(h1 + "/24") // XXX
		if r != nil {
			return addrs
		}
		ip2 := net.ParseIP(ip)
		if ip1 == nil || ip2 == nil {
			return raddrs
		}

		// See if ip is local (i.e., on same subnet) as a
		if ipnet1.Contains(ip2) {
			l = i
			local = true
			break
		}

		// See if a is a host address
		_, pnet2, r := net.ParseCIDR("10.0.0.0/8") // XXX fix for aws
		if r != nil {
			return raddrs
		}
		if !pnet2.Contains(ip1) {
			p = i
		}

	}
	if local {
		swap(raddrs, l)
	} else {
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
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return "", err
	}
	if host == "::" {
		ip, err := LocalIP()
		if err != nil {
			return "", err
		}
		addr = net.JoinHostPort(ip, port)
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
		if !strings.HasPrefix(i.String(), "172.") {
			return i.String(), nil
		}
	}

	if len(ips) == 0 {
		return "", fmt.Errorf("LocalIP: no IP")
	}

	return ips[len(ips)-1].String(), nil
}
