use std::env;
use std::process::Command;
use std::os::unix::process::CommandExt;
use std::time::{SystemTime, UNIX_EPOCH};

use json;

use serde::{Serialize, Deserialize};
use serde_yaml::{self};

#[derive(Debug, Serialize, Deserialize)]
struct Config {
    allowed: Vec<String>,
    cond_allowed: Vec<Cond>
}
#[derive(Debug, Serialize, Deserialize)]
struct Cond {
    index: u32,
    op1: u64,
    op: String,
}

fn main() {
    let exec_time = env::var("SIGMA_EXEC_TIME").unwrap_or("".to_string());
    let exec_time_micro: u64 = exec_time.parse().unwrap_or(0);

    eprintln!("exec_uproc SIGMA_EXEC_TIME {}", exec_time_micro);

    let cfg = env::var("SIGMACONFIG").unwrap_or("".to_string());
    if cfg != "" {
        let parsed = json::parse(&cfg).unwrap();
        println!("{}", parsed);
    }

    let pn = env::args().nth(1).expect("no program");

    seccomp_proc();
    
    let new_args: Vec<_> = std::env::args_os().skip(1).collect();
    let mut cmd = Command::new(pn);
    
    
    let now = SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .expect("Time went backwards");
     
    env::set_var("SIGMA_EXEC_TIME", now.as_micros().to_string());


    cmd.args(new_args).exec();
}

fn seccomp_proc() {
    let yaml_str = r#"
allowed:
  - accept4
  - access
  - arch_prctl # Enabled by Docker on AMD64, which is the only architecture we're running on at the moment.
  - bind
  - brk
  - clone3 # Needed by Go runtime on old versions of docker. See https://github.com/moby/moby/issues/42680
  - close
  - connect
  - epoll_create1
  - epoll_ctl
  - epoll_ctl_old
  - epoll_pwait
  - epoll_pwait2
  - execve
  - exit_group
  - fcntl
  - fstat
  - fsync
  - futex
  - getdents64
  - getpeername
  - getpid
  - getrandom
  - getrlimit
  - getsockname
  - getsockopt
  - gettid
  - listen
  - lseek
  - madvise
  - mkdirat
  - mmap
  - mprotect
  - munmap
  - nanosleep
  - newfstatat
  - openat
  - open
  - pipe2
  - pread64
  - prlimit64
  - read
  - readlinkat
  - recvfrom
  - restart_syscall
  - rt_sigaction
  - rt_sigprocmask
  - rt_sigreturn
  - sched_getaffinity
  - sched_yield
  - sendto
  - setitimer
  - set_robust_list
  - set_tid_address
  - setsockopt
  - sigaltstack
  - sync
  - timer_create
  - timer_delete
  - timer_settime
  - tgkill
  - write
  - writev
# Needed for MUSL/Alpine
  - readlink

cond_allowed:
  - socket: # Allowed by docker if arg0 != 40 (disallows AF_VSOCK).
    index: 0
    op1: 40
    op: "SCMP_CMP_NE"
  - clone:
    index: 0
    op1: 0x7E020000
    op: "SCMP_CMP_MASKED_EQ"
"#;

    let cfg: Config = serde_yaml::from_str(&yaml_str).expect("Couldn't read yaml str");
    
    println!("{:?}", cfg);
}
