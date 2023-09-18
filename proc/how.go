package proc

import (
	"log"
)

type Thow uint32

const (
	HSCHEDD Thow = iota + 1 // spawned as a sigmos proc
	HLINUX                  // spawned as a linux process
	HDOCKER                 // spawned as a container
)

func (h Thow) String() string {
	switch h {
	case HSCHEDD:
		return "schedd"
	case HLINUX:
		return "linux"
	case HDOCKER:
		return "docker"
	default:
		log.Fatalf("FATAL: Unknown how %v", int(h))
		return "unknown how"
	}
}
