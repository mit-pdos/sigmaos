package proc

import (
	"path"
)

/*
 * Proc Directory structure:
 *
 * /
 * |- procd
 * |  |
 * |  |- x.x.x.x
 * |  |  |
 * |  |  |- pids
 * |  |     |
 * |  |     |- 1000 // Proc mounts this directory as procdir
 * |  |         |
 * |  |         |- evict-sem
 * |  |         |- exit-sem
 * |  |         |- children
 * |  |            |- 1001 // Child mounts this directory as procdir/parent
 * |  |               |- start-sem
 * |  |               |- exit-status
 * |  |               |- procdir -> /procd/y.y.y.y/pids/1001 // Symlink to child's procdir.
 * |  |                  |- ...
 * |  |
 * |  |- y.y.y.y
 * |     |
 * |     |- pids
 * |        |
 * |        |- 1001
 * |            |
 * |            |- parentdir -> /procd/x.x.x.x/pids/1000/children/1001 // Mount of subdir of parent proc.
 * |            |- ...
 * |
 * |- kernel-pids // Only for kernel procs such as s3, ux, procd, ...
 *    |
 *    |- fsux-2000
 *       |
 *       |- kernel-proc // Only present if this is a kernel proc.
 *       |- ... // Same directory structure as regular procs
 */

const (
	// name for dir where procs live. May not refer to name/pids
	// because proc.PidDir may change it.  A proc refers to itself
	// using "pids/<pid>", where pid is the proc's PID.
	PIDS      = "pids" // TODO: make this explicitly kernel PIDs only
	KPIDS     = "kpids"
	PROCDIR   = "procdir"
	PARENTDIR = "parentdir"

	// Files/directories in "pids/<pid>":
	START_SEM   = "start-sem"
	EXIT_SEM    = "exit-sem"
	EVICT_SEM   = "evict-sem"
	EXIT_STATUS = "exit-status"
	CHILDREN    = "children"    // directory with children's pids and symlinks
	KERNEL_PROC = "kernel-proc" // Only present if this is a kernel proc
)

func GetChildProcDir(cpid string) string {
	return path.Join(PROCDIR, CHILDREN, cpid, PROCDIR)
}
