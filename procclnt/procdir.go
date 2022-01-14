package procclnt

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
 * |  |     |- 1000
 * |  |         |
 * |  |         |- start-sem
 * |  |         |- evict-sem
 * |  |         |- status-pipe
 * |  |         |- children
 * |  |            |- 1001 // Child mounts this directory as procdir/parent
 * |  |               |- run-sem
 * |  |               |- procdir -> /procd/y.y.y.y/pids/1001 // Symlink to child's procdir.
 * |  |                  |- ...
 * |  |
 * |  |- y.y.y.y
 * |     |
 * |     |- pids
 * |        |
 * |        |- 1001
 * |            |
 * |            |- parent -> /procd/x.x.x.x/pids/1000/children/1001 // Mount of subdir of parent proc.
 * |            |- ...
 * |
 * |- pids // Only for kernel procs such as s3, ux, procd, ...
 *    |
 *    |- fsux-2000
 *       |
 *       |- kernel-proc // Only present if this is a kernel proc.
 *       |- ... // Same directory structure as regular procs
 */
