//
// Code from https://lk4d4.darth.io/posts/unpriv4/
//

// You should run this binary with suid set.
package main

import (
	"fmt"
	"log"
	"math/rand"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/vishvananda/netlink"

	"sigmaos/container"
)

const (
	bridgeName = "sigmab"
	vethPrefix = "sb"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

func createBridge() error {
	log.Printf("create bridge %v\n", bridgeName)
	// try to get bridge by name, if it already exists then just exit
	_, err := net.InterfaceByName(bridgeName)
	if err == nil {
		return nil
	}
	if !strings.Contains(err.Error(), "no such network interface") {
		return err
	}
	// create *netlink.Bridge object
	la := netlink.NewLinkAttrs()
	la.Name = bridgeName
	br := &netlink.Bridge{LinkAttrs: la}
	if err := netlink.LinkAdd(br); err != nil {
		return fmt.Errorf("bridge creation: %v", err)
	}
	// set up ip addres for bridge
	addr, err := netlink.ParseAddr(container.IPAddr)
	if err != nil {
		return fmt.Errorf("parse address %s: %v", container.IPAddr, err)
	}
	if err := netlink.AddrAdd(br, addr); err != nil {
		return fmt.Errorf("add address %v to bridge: %v", addr, err)
	}
	// sets up bridge ( ip link set dev sigmab up )
	if err := netlink.LinkSetUp(br); err != nil {
		return err
	}

	// XXX add and delete fix iptables
	// iptables --append FORWARD --in-interface sigmab --out-interface sigmab --jump ACCEPT
	// iptables --append FORWARD --in-interface wlp2s0 --out-interface sigmab --jump ACCEPT
	// iptables --append FORWARD --in-interface sigmab --out-interface wlp2s0 --jump ACCEPT
	// iptables --append POSTROUTING --table nat --out-interface wlp2s0 --jump MASQUERADE

	return nil
}

func createVethPair(pid int) error {
	// get bridge to set as master for one side of veth-pair
	br, err := netlink.LinkByName(bridgeName)
	if err != nil {
		return err
	}
	// generate names for interfaces
	x1, x2 := rand.Intn(10000), rand.Intn(10000)
	parentName := fmt.Sprintf("%s%d", vethPrefix, x1)
	peerName := fmt.Sprintf("%s%d", vethPrefix, x2)

	log.Printf("createVethPair parent %v peer %v\n", parentName, peerName)

	// create *netlink.Veth
	la := netlink.NewLinkAttrs()
	la.Name = parentName
	la.MasterIndex = br.Attrs().Index
	vp := &netlink.Veth{LinkAttrs: la, PeerName: peerName}
	if err := netlink.LinkAdd(vp); err != nil {
		return fmt.Errorf("veth pair creation %s <-> %s: %v", parentName, peerName, err)
	}
	// get peer by name to put it to namespace
	peer, err := netlink.LinkByName(peerName)
	if err != nil {
		return fmt.Errorf("get peer interface: %v", err)
	}
	// put peer side to network namespace of specified PID
	if err := netlink.LinkSetNsPid(peer, pid); err != nil {
		return fmt.Errorf("move peer to ns of %d: %v", pid, err)
	}
	if err := netlink.LinkSetUp(vp); err != nil {
		return err
	}
	return nil
}

func delBridge() error {
	cmd := exec.Command("ip", "link", "delete", "dev", bridgeName)
	if _, err := cmd.CombinedOutput(); err != nil {
		return err
	}
	return nil
}

func main() {
	if len(os.Args) != 3 {
		log.Fatalf("%s: too few arguments <up> <pid>\n", os.Args[0])
	}
	pid, err := strconv.Atoi(os.Args[2])
	if err != nil {
		log.Fatal(err)
	}
	switch os.Args[1] {
	case "up":
		if err := createBridge(); err != nil {
			log.Fatal(err)
		}
		if err := createVethPair(pid); err != nil {
			log.Fatal(err)
		}
	case "down":
		if err := delBridge(); err != nil {
			log.Fatal(err)
		}
	}
}
