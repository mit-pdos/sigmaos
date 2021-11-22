package kernel

import (
	"log"
	"os/exec"
	"path"
	"syscall"

	"ulambda/fslib"
	"ulambda/named"
	"ulambda/proc"
	"ulambda/procd"
	"ulambda/sync"
)

type System struct {
	bin       string
	namedAddr []string
	fss3d     []*exec.Cmd
	fss3dPids []string
	fsuxd     []*exec.Cmd
	fsuxdPids []string
	procd     []*exec.Cmd
	procdPids []string
	dbd       []*exec.Cmd
	dbdPids   []string
	*fslib.FsLib
}

func MakeSystem(bin string, namedAddr []string) *System {
	s := &System{}
	s.bin = bin
	s.namedAddr = namedAddr
	s.FsLib = fslib.MakeFsLibAddr("kernel", namedAddr)
	return s
}

// Boot a full system
func (s *System) Boot() error {
	err := s.BootFsUxd()
	if err != nil {
		return err
	}
	err = s.BootFss3d()
	if err != nil {
		return err
	}
	err = s.BootProcd()
	if err != nil {
		return err
	}
	err = s.BootDbd()
	if err != nil {
		return err
	}
	return nil
}

func (s *System) BootFsUxd() error {
	// Create boot cond
	pid := "fsuxd-" + proc.GenPid()
	fsuxdStartCond := sync.MakeCond(s.FsLib, path.Join(named.BOOT, pid), nil, true)
	fsuxdStartCond.Init()
	var err error
	fsuxd, err := procd.Run(pid, s.bin, "bin/kernel/fsuxd", s.namedAddr, []string{})
	s.fsuxd = append(s.fsuxd, fsuxd)
	if err != nil {
		return err
	}
	// Wait for boot
	fsuxdStartCond.Wait()
	s.fsuxdPids = append(s.fsuxdPids, pid)
	return nil
}

func (s *System) BootFss3d() error {
	// Create boot cond
	pid := "fss3d-" + proc.GenPid()
	fss3dStartCond := sync.MakeCond(s.FsLib, path.Join(named.BOOT, pid), nil, true)
	fss3dStartCond.Init()
	var err error
	fss3d, err := procd.Run(pid, s.bin, "bin/kernel/fss3d", s.namedAddr, []string{})
	s.fss3d = append(s.fss3d, fss3d)
	if err != nil {
		return err
	}
	// Wait for boot
	fss3dStartCond.Wait()
	s.fss3dPids = append(s.fss3dPids, pid)
	return nil
}

func (s *System) BootProcd() error {
	// Create boot cond
	pid := named.PROCDDIR + "-" + proc.GenPid()
	procdStartCond := sync.MakeCond(s.FsLib, path.Join(named.BOOT, pid), nil, true)
	procdStartCond.Init()
	var err error
	procd, err := procd.Run(pid, s.bin, "bin/kernel/procd", s.namedAddr, []string{s.bin})
	s.procd = append(s.procd, procd)
	if err != nil {
		return err
	}
	// Wait for boot
	procdStartCond.Wait()
	s.procdPids = append(s.procdPids, pid)
	return nil
}

func (s *System) BootDbd() error {
	// Create dbd cond
	pid := "dbd-" + proc.GenPid()
	dbdStartCond := sync.MakeCond(s.FsLib, path.Join(named.BOOT, pid), nil, true)
	dbdStartCond.Init()
	var err error
	dbd, err := procd.Run(pid, s.bin, "bin/kernel/dbd", s.namedAddr, []string{})
	s.dbd = append(s.dbd, dbd)
	if err != nil {
		return err
	}
	// Wait for boot
	dbdStartCond.Wait()
	s.dbdPids = append(s.dbdPids, pid)
	return nil
}

func (s *System) shutdownAll(mdir string, pids []string) error {
	for _, pid := range pids {
		err := s.FsLib.ShutdownFs(path.Join(mdir, pid))
		if err != nil {
			log.Printf("shutdown err %v\n", err)
			return err
		}
	}
	return nil
}

func (s *System) KillOne(srv string) error {
	var err error
	switch srv {
	case named.PROCD:
		if len(s.procd) > 0 {
			err = syscall.Kill(-s.procd[0].Process.Pid, syscall.SIGKILL)
			//			err = s.procd[0].Process.Kill()
			if err == nil {
				s.procd[0].Wait()
				s.procd = s.procd[1:]
			} else {
				log.Fatalf("Procd kill failed %v\n", err)
			}
		}
	default:
		log.Fatalf("Unkown server type in System.KillOne: %v", srv)
	}
	return nil
}

func (s *System) Shutdown() {
	if len(s.fss3d) != 0 {
		err := s.shutdownAll(named.S3, s.fss3dPids)
		if err != nil {
			log.Printf("S3 shutdown %v\n", err)
		}
		for _, d := range s.fss3d {
			d.Wait()
		}
	}
	if len(s.fsuxd) != 0 {
		err := s.shutdownAll(named.UX, s.fsuxdPids)
		if err != nil {
			log.Printf("Ux shutdown %v\n", err)
		}
		for _, d := range s.fsuxd {
			d.Wait()
		}
	}
	if len(s.procd) != 0 {
		err := s.shutdownAll(named.PROCD, s.procdPids)
		if err != nil {
			log.Printf("Procds shutdown %v\n", err)
		}
		for _, d := range s.procd {
			d.Wait()
		}
	}
	if len(s.dbd) != 0 {
		err := s.shutdownAll(named.DB, s.dbdPids)
		if err != nil {
			log.Printf("Db shutdown %v\n", err)
		}
		for _, d := range s.dbd {
			d.Wait()
		}
	}
}
