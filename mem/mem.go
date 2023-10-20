package mem

import (
	"io/ioutil"
	"strconv"
	"strings"

	db "sigmaos/debug"
	"sigmaos/proc"
)

var totalMem proc.Tmem = 0

func getMem(pat string) proc.Tmem {
	b, err := ioutil.ReadFile("/proc/meminfo")
	if err != nil {
		db.DFatalf("Can't read /proc/meminfo: %v", err)
	}
	lines := strings.Split(string(b), "\n")
	for _, l := range lines {
		if strings.Contains(l, pat) {
			s := strings.Split(l, " ")
			kbStr := s[len(s)-2]
			kb, err := strconv.Atoi(kbStr)
			if err != nil {
				db.DFatalf("Couldn't convert MemTotal: %v", err)
			}
			return proc.Tmem(kb / 1024)
		}
	}
	db.DFatalf("Couldn't find total mem")
	return 0
}

// Total amount of memory, in MB.
func GetTotalMem() proc.Tmem {
	if totalMem == 0 {
		totalMem = getMem("MemTotal")
	}
	return totalMem
}

// Available amount of memory, in MB.
func GetAvailableMem() proc.Tmem {
	return getMem("MemAvailable")
}
