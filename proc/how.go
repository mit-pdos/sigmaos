package proc

import (
	"log"
	"runtime/debug"
)

type Thow uint32

const (
	HMSCHED Thow = iota + 1 // spawned as a sigmos proc
	HLINUX                  // spawned as a linux process
	HDOCKER                 // spawned as a container
	TEST                    // test program (not spawned)
	BOOT                    // boot program (not spawned)
)

func (h Thow) String() string {
	switch h {
	case HMSCHED:
		return "msched"
	case HLINUX:
		return "linux"
	case HDOCKER:
		return "docker"
	case BOOT:
		return "boot"
	case TEST:
		return "test"
	default:
		b := debug.Stack()
		log.Fatalf("FATAL: Unknown how %v stack:\n%v", int(h), string(b))
		return "unknown how"
	}
}
