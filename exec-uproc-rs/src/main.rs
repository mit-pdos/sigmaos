use std::env;
use std::fs;
use std::process::Command;
use std::os::unix::process::CommandExt;
use std::time::{SystemTime, UNIX_EPOCH};

use json;

use serde::{Serialize, Deserialize};
use serde_yaml::{self};

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

    jail_proc().expect("jail failed");
    setcap_proc().expect("set caps failed");
    seccomp_proc().expect("seccomp failed");
    
    let new_args: Vec<_> = std::env::args_os().skip(1).collect();
    let mut cmd = Command::new(pn);
    
    let now = SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .expect("Time went backwards");
     
    env::set_var("SIGMA_EXEC_TIME", now.as_micros().to_string());

    cmd.args(new_args).exec();
}

fn jail_proc() ->  Result<(), Box<dyn std::error::Error>> {
    extern crate sys_mount;
    use sys_mount::{Mount, MountFlags, unmount, UnmountFlags};

    let old_root_mnt = "oldroot";
    const DIRS: &'static [&'static str] = &["", "oldroot", "lib", "usr", "lib64", "etc", "sys", "dev", "proc", "seccomp", "bin", "bin2", "tmp", "cgroup"];
    
    let newroot = "/home/sigmaos/jail/";
    let sigmahome = "/home/sigmaos/";

    println!("make dirs\n");
    
    for d in DIRS.iter() {
        let path : String = newroot.to_owned();
        fs::create_dir_all(path+d)?;
    }

    println!("make dirs done\n");
    
    Mount::builder()
        .fstype("")
        .flags(MountFlags::BIND | MountFlags::REC)
        .mount(newroot, newroot)?;

    println!("mount newroot\n");
    
    env::set_current_dir(newroot)?;

    Mount::builder()
        .fstype("none")
        .flags(MountFlags::BIND | MountFlags::RDONLY)
        .mount("/lib", "lib")?;

    println!("mount lib\n");
    
    Mount::builder()
        .fstype("none")
        .flags(MountFlags::BIND | MountFlags::RDONLY)
        .mount("/lib64", "lib64")?;

    println!("mount lib64\n");
    
    let mut shome : String = sigmahome.to_owned();

    Mount::builder()
        .fstype("proc")
        .mount("proc", "proc")?;

    println!("mount proc\n");
        
    Mount::builder()
        .fstype("none")
        .flags(MountFlags::BIND | MountFlags::RDONLY)
        .mount(shome+"bin/user", "bin")?;

    println!("mount bin\n");
    
    shome = sigmahome.to_owned();
    Mount::builder()
        .fstype("none")
        .flags(MountFlags::BIND | MountFlags::RDONLY)
        .mount(shome+"bin/kernel", "bin2")?;

    println!("mount kernel\n");
    
    shome = sigmahome.to_owned();
    Mount::builder()
        .fstype("none")
        .flags(MountFlags::BIND | MountFlags::RDONLY)
        .mount(shome+"seccomp", "../seccomp")?;

    println!("mount seccomp\n");
    
    Mount::builder()
        .fstype("none")
        .flags(MountFlags::BIND)
        .mount("/cgroup", "cgroup")?;

    println!("mount cgroup\n");
    
    // XXX todo: mount perf output
    
    Mount::builder()
        .fstype("none")
        .flags(MountFlags::BIND | MountFlags::RDONLY)
        .mount("/usr", "usr")?;

    println!("mount usr\n");
    
    Mount::builder()
        .fstype("sysfs")
        .flags(MountFlags::BIND | MountFlags::RDONLY)
        .mount("/sys", "sys")?;

    println!("mount sys\n");
    
    Mount::builder()
        .fstype("none")
        .flags(MountFlags::BIND | MountFlags::RDONLY)
        .mount("/dev", "dev")?;

    println!("mount dev\n");
    
    Mount::builder()
        .fstype("none")
        .flags(MountFlags::BIND | MountFlags::RDONLY)
        .mount("/etc", "etc")?;

    println!("mount etc\n");
    
    // XXX pivot_root(newroot.as_os_str(), rootfs.join("oldroot").as_os_str())?;

    env::set_current_dir("/")?;

    unmount(old_root_mnt, UnmountFlags::DETACH)?;

    // XXX Remove(old_root_mnt)

    Ok(())
}

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

fn seccomp_proc()  -> Result<(), Box<dyn std::error::Error>> {
    use libseccomp::*;

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

    let cfg: Config = serde_yaml::from_str(&yaml_str)?;
    let mut filter = ScmpFilterContext::new_filter(ScmpAction::Errno(1))?;
    for name in cfg.allowed {
        let syscall = ScmpSyscall::from_name(&name)?;
        filter.add_rule(ScmpAction::Allow, syscall)?;
    }
    filter.load()?;
    Ok(())
}

fn setcap_proc() -> Result<(), Box<dyn std::error::Error>> {
    use caps::{CapSet, Capability};

    let cur = caps::read(None, CapSet::Permitted)?;
    println!("Current permitted caps: {:?}.", cur);
    let cur = caps::read(None, CapSet::Effective)?;
    println!("Current effective caps: {:?}.", cur);

    Ok(())
}
