package seccomp

// helpful ref: https://eigenstate.org/notes/seccomp

//
// The following system calls allows memfsd to run.  This list should
// probably be paired down further or limited based on arguments.
//
// Don't allow "openat", because this would allow a process to read
// local resources. For example, adding it would enable fsuxd to work,
// and we want arbitrary processes not to read/write local files.
//
// allow exit_group() to exit a process. don't use _exit because it
// just terminates a thread.
//

var whitelist = []string{
	"accept4",
	"access",
	"arch_prctl",
	"bind",
	"brk",
	"clone",
	"close",
	"connect",
	"epoll_create1",
	"epoll_ctl",
	"epoll_pwait",
	"execve",
	"exit_group",
	"fcntl",
	"futex",
	"getdents64",
	"getpeername",
	"getpid",
	"getsockname",
	"getsockopt",
	"gettid",
	"listen",
	"mmap",
	"mprotect",
	"munmap",
	"nanosleep",
	"newfstatat",
	"openat", // XXX Needed for kv test
	"open",   // XXX Needed to open /dev/urandom
	"pipe2",
	"pread64",
	"prlimit64",
	"read",
	"readlinkat",
	"recvfrom",
	"rt_sigaction",
	"rt_sigprocmask",
	"rt_sigreturn",
	"sched_getaffinity",
	"sched_yield",
	"sendto",
	"set_robust_list",
	"set_tid_address",
	"setsockopt",
	"sigaltstack",
	"socket",
	"write",
}
