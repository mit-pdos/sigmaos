package procd

import (
	"io/ioutil"
	"strconv"
	"strings"

	db "ulambda/debug"
	"ulambda/proc"
)

func getMemTotal() proc.Tmem {
	b, err := ioutil.ReadFile("/proc/meminfo")
	if err != nil {
		db.DFatalf("Can't read /proc/meminfo: %v", err)
	}
	lines := strings.Split(string(b), "\n")
	for _, l := range lines {
		if strings.Contains(l, "MemTotal") {
			s := strings.Split(l, " ")
			kbStr := s[len(s)-2]
			kb, err := strconv.Atoi(kbStr)
			if err != nil {
				db.DFatalf("Couldn't convert MemTotal: %v", err)
			}
			return proc.Tmem(kb / 1000)
		}
	}
	db.DFatalf("Couldn't find total mem")
	return 0
}

func (pd *Procd) hasEnoughMemL(p *proc.Proc) bool {
	return pd.memAvail >= p.Mem
}

func (pd *Procd) allocMemL(p *proc.Proc) {
	pd.memAvail -= p.Mem
}

func (pd *Procd) freeMemL(p *proc.Proc) {
	pd.memAvail += p.Mem
}
