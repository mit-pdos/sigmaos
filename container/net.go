package container

import (
	"fmt"
	"math/rand"
	"net"
	"os/exec"
	"strconv"
	"time"

	"github.com/vishvananda/netlink"

	db "sigmaos/debug"
)

//
// Network setup for a kernel container.  Each kernel gets its own
// network address.
//

const (
	IPFormat = "10.100.%d.%d/24"
	SCNETBIN = "/usr/bin/scnet"
)

func mkIpNet() (string, string) {
	rand.Seed(time.Now().UnixNano())
	net := rand.Intn(253) + 2
	ip := fmt.Sprintf(IPFormat, net, rand.Intn(253)+2)
	r := fmt.Sprintf(IPFormat, net, 1)
	return ip, r
}

func mkScnet(pid int, rip, realm string) error {
	cmd := exec.Command(SCNETBIN, "up", strconv.Itoa(pid), rip, realm)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("scnet: out: %s, err: %v", out, err)
	}
	return nil
}

func DelScnet(pid int, realm string) error {
	cmd := exec.Command(SCNETBIN, "down", strconv.Itoa(pid), realm)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("scnet: out: %s, err: %v", out, err)
	}
	return nil
}

//
// Setup network inside container
//

func setupScnet(ip string) error {
	db.DPrintf(db.CONTAINER, "SetupScnet %v\n", ip)
	lnk, err := waitScnet()
	if err != nil {
		db.DPrintf(db.CONTAINER, "wait failed err %v\n", err)
		return err
	}
	db.DPrintf(db.CONTAINER, "wait link %v\n", lnk.Attrs().Name)
	if err := confScnet(lnk, ip); err != nil {
		db.DPrintf(db.CONTAINER, "setup failed err %v\n", err)
		return err
	}
	return nil
}

func waitScnet() (netlink.Link, error) {
	const NSEC = 5
	db.DPrintf(db.CONTAINER, "Wait for network interface\n")
	start := time.Now()
	for {
		if time.Since(start) > NSEC*time.Second {
			return nil, fmt.Errorf("failed to find veth interface in %d seconds", NSEC)
		}
		lst, err := netlink.LinkList()
		if err != nil {
			return nil, err
		}
		for _, l := range lst {
			// found "veth" interface
			if l.Type() == "veth" {
				return l, nil
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func confScnet(lnk netlink.Link, ip string) error {
	db.DPrintf(db.CONTAINER, "Setup network interface\n")

	// up loopback
	lo, err := netlink.LinkByName("lo")
	if err != nil {
		return fmt.Errorf("lo interface: %v", err)
	}
	if err := netlink.LinkSetUp(lo); err != nil {
		return fmt.Errorf("up veth: %v", err)
	}

	// up and configure lnk
	if err := netlink.LinkSetUp(lnk); err != nil {
		return fmt.Errorf("up veth: %v", err)
	}
	addr, err := netlink.ParseAddr(ip)
	if err != nil {
		return fmt.Errorf("ParseAddr: %v", err)
	}
	db.DPrintf(db.CONTAINER, "addr lnk %v %v\n", addr, lnk.Attrs().Name)
	if err := netlink.AddrAdd(lnk, addr); err != nil {
		return err
	}
	i, _, err := net.ParseCIDR(ip)
	if err != nil {
		return fmt.Errorf("ParseCIDR %v error %v", ip, err)
	}
	gw := i.To4()
	gw[3] = 1
	dr := netlink.Route{Gw: gw, Dst: nil}
	if err := netlink.RouteAdd(&dr); err != nil {
		return err
	}
	db.DPrintf(db.CONTAINER, "route gw %v %v\n", gw, dr)
	return nil
}
