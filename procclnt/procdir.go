package procclnt

/*
 * Directory structure:
 *
 * /procd
 * |
 * |- x.x.x.x
 *    |
 *    |- pids
 *       |
 *       |- 1000
 *           |
 *           |- start-sem
 *           |- evict-sem
 *           |- status-pipe
 *           |- kernel-proc // Only present if this is a kernel proc.
 *           |- children
 *              |- 1001
 *                 |- run-sem
 *                 |- procdir -> /procd/y.y.y.y/pids/1001 // This is a symbolic link
 */
