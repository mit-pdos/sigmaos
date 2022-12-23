package proc

import (
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/vishvananda/netlink"

	sp "sigmaos/sigmap"
)

// To run kernel procs
func RunKernelProc(p *Proc, namedAddr []string) (*exec.Cmd, error) {
	p.FinalizeEnv("NONE")
	env := p.GetEnv()
	env = append(env, "NAMED="+strings.Join(namedAddr, ","))
	env = append(env, "SIGMAPROGRAM="+p.Program)

	cmd := exec.Command(path.Join(sp.PRIVILEGED_BIN, p.Program), p.Args...)
	// Create a process group ID to kill all children if necessary.
	// XXX merge with namespace
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
		Cloneflags: syscall.CLONE_NEWUTS |
			syscall.CLONE_NEWNS |
			syscall.CLONE_NEWIPC |
			syscall.CLONE_NEWNET |
			syscall.CLONE_NEWPID |
			syscall.CLONE_NEWUSER,
		UidMappings: []syscall.SysProcIDMap{
			{
				ContainerID: 0,
				HostID:      os.Getuid(),
				Size:        1,
			},
		},
		GidMappings: []syscall.SysProcIDMap{
			{
				ContainerID: 0,
				HostID:      os.Getgid(),
				Size:        1,
			},
		},
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ(), env...)
	if err := cmd.Start(); err != nil {
		log.Printf("start failed err %v\n", err)
		return nil, err
	}
	log.Printf("enter %v\n", cmd.Process.Pid)
	if err := enterScnet(cmd.Process.Pid); err != nil {
		log.Printf("enter failed err %v\n", err)
		return nil, err
	}
	return cmd, nil
}

func SetupScnet(ip string) error {
	log.Printf("SetupScnet %v\n", ip)
	lnk, err := waitScnet()
	if err != nil {
		log.Printf("wait failed err %v\n", err)
		return err
	}
	log.Printf("wait link %v\n", lnk.Attrs().Name)
	if err := setupScnet(lnk, ip); err != nil {
		log.Printf("setup failed err %v\n", err)
		return err
	}
	return nil
}

func enterScnet(pid int) error {
	cmd := exec.Command("/usr/bin/scnet", strconv.Itoa(pid))
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("scnet: out: %s, err: %v", out, err)
	}
	return nil
}

func waitScnet() (netlink.Link, error) {
	const NSEC = 5
	fmt.Printf("Wait for network interface\n")
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

func setupScnet(lnk netlink.Link, ip string) error {
	fmt.Printf("Setup network interface\n")

	cidr := ip + "/24"

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
	addr, err := netlink.ParseAddr(cidr)
	if err != nil {
		return fmt.Errorf("ParseAddr: %v", err)
	}
	log.Printf("addr lnk %v %v\n", addr, lnk.Attrs().Name)
	if err := netlink.AddrAdd(lnk, addr); err != nil {
		return err
	}
	i, _, err := net.ParseCIDR(cidr)
	if err != nil {
		return fmt.Errorf("ParseCIDR %v error %v", cidr, err)
	}
	gw := i.To4()
	gw[3] = 1
	dr := netlink.Route{Gw: gw, Dst: nil}
	log.Printf("route gw %v %v\n", gw, dr)
	if err := netlink.RouteAdd(&dr); err != nil {
		return err
	}
	log.Printf("route gw %v %v\n", gw, dr)
	return nil
}
