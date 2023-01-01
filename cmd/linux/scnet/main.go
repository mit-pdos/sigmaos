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

	"github.com/coreos/go-iptables/iptables"
	"github.com/vishvananda/netlink"

	"sigmaos/container"
)

const (
	BRIDGENAME = "sigmab"
	vethPrefix = "sb"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

func bridgeName(realm string) string {
	return BRIDGENAME
}

func addIpTables() error {
	ipt, err := iptables.NewWithProtocol(iptables.ProtocolIPv4)
	if err != nil {
		return err
	}
	args := []string{"-i", BRIDGENAME, "-o", BRIDGENAME, "-j", "ACCEPT"}
	if err := ipt.Insert("filter", "FORWARD", 1, args...); err != nil {
		return fmt.Errorf("iptables command err %s", err.Error())
	}
	log.Printf("add rule %v\n", args)
	args = []string{"-i", "wlp2s0", "-o", BRIDGENAME, "-j", "ACCEPT"}
	log.Printf("add rule %v\n", args)
	if err := ipt.Insert("filter", "FORWARD", 1, args...); err != nil {
		return fmt.Errorf("iptables command err %s", err.Error())
	}
	args = []string{"-i", BRIDGENAME, "-o", "wlp2s0", "-j", "ACCEPT"}
	log.Printf("add rule %v\n", args)
	if err := ipt.Insert("filter", "FORWARD", 1, args...); err != nil {
		return fmt.Errorf("iptables command err %s", err.Error())
	}
	return nil
}

func delIpTables() error {
	ipt, err := iptables.NewWithProtocol(iptables.ProtocolIPv4)
	if err != nil {
		return err
	}
	args := []string{"-i", BRIDGENAME, "-o", BRIDGENAME, "-j", "ACCEPT"}
	log.Printf("del rule %v\n", args)
	if err := ipt.Delete("filter", "FORWARD", args...); err != nil {
		return fmt.Errorf("iptables command err %s", err.Error())
	}
	args = []string{"-i", "wlp2s0", "-o", BRIDGENAME, "-j", "ACCEPT"}
	log.Printf("del rule %v\n", args)
	if err := ipt.Delete("filter", "FORWARD", args...); err != nil {
		return fmt.Errorf("iptables command err %s", err.Error())
	}
	args = []string{"-i", BRIDGENAME, "-o", "wlp2s0", "-j", "ACCEPT"}
	log.Printf("add rule %v\n", args)
	if err := ipt.Delete("filter", "FORWARD", args...); err != nil {
		return fmt.Errorf("iptables command err %s", err.Error())
	}
	return nil
}

func createBridge(realm string) error {
	log.Printf("create bridge %v %s\n", bridgeName(realm), realm)
	// try to get bridge by name, if it already exists then just exit
	_, err := net.InterfaceByName(bridgeName(realm))
	if err == nil {
		return nil
	}
	if !strings.Contains(err.Error(), "no such network interface") {
		return err
	}
	// create *netlink.Bridge object
	la := netlink.NewLinkAttrs()
	la.Name = bridgeName(realm)
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
	// if err := addIpTables(); err != nil {
	// 	return err
	// }
	return nil
}

func createVethPair(pid int, realm string) error {
	// get bridge to set as master for one side of veth-pair
	br, err := netlink.LinkByName(bridgeName(realm))
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

func delBridge(realm string) error {
	cmd := exec.Command("ip", "link", "delete", "dev", bridgeName(realm))
	if _, err := cmd.CombinedOutput(); err != nil {
		return err
	}
	// if err := delIpTables(); err != nil {
	// 	return err
	// }
	return nil
}

func main() {
	if len(os.Args) != 4 {
		log.Fatalf("%s: too few arguments <up> <pid> <realm>\n", os.Args[0])
	}
	pid, err := strconv.Atoi(os.Args[2])
	if err != nil {
		log.Fatal(err)
	}
	switch os.Args[1] {
	case "up":
		if err := createBridge(os.Args[3]); err != nil {
			log.Fatalf("%s: create bridge err %v\n", os.Args[0], err)
		}
		if err := createVethPair(pid, os.Args[3]); err != nil {
			log.Fatalf("%s: pair err %v\n", os.Args[0], err)
		}
	case "down":
		if err := delBridge(os.Args[3]); err != nil {
			log.Fatalf("%s: scnet down err %v\n", os.Args[0], err)
		}
	}
}
