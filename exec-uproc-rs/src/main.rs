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
    let parsed = json::parse(&cfg).unwrap();
    
    eprintln!("Cfg: {}", parsed);

    let program = env::args().nth(1).expect("no program");
    let pid = parsed["pidStr"].as_str().unwrap_or("pid not");

    jail_proc(pid).expect("jail failed");
    setcap_proc().expect("set caps failed");
    seccomp_proc().expect("seccomp failed");
    
    let new_args: Vec<_> = std::env::args_os().skip(2).collect();
    let mut cmd = Command::new(program.clone());
    
    let now = SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .expect("Time went backwards");
     
    env::set_var("SIGMA_EXEC_TIME", now.as_micros().to_string());

    eprintln!("exec: {} {:?}", program, new_args);
    
    let err = cmd.args(new_args).exec();
    
    eprintln!("err: {}", err);
}

fn jail_proc(pid : &str) ->  Result<(), Box<dyn std::error::Error>> {
    extern crate sys_mount;
    use sys_mount::{Mount, MountFlags, unmount, UnmountFlags};
    use nix::unistd::{pivot_root};

    let old_root_mnt = "oldroot";
    const DIRS: &'static [&'static str] = &["", "oldroot", "lib", "usr", "lib64", "etc", "sys", "dev", "proc", "seccomp", "bin", "bin2", "tmp", "cgroup"];
    
    let newroot = "/home/sigmaos/jail/";
    let sigmahome = "/home/sigmaos/";
    let newroot_pn: String = newroot.to_owned() + pid + "/";
    
    for d in DIRS.iter() {
        let path : String = newroot_pn.to_owned();
        fs::create_dir_all(path+d)?;
    }

    eprintln!("mount newroot {}", newroot_pn);
    
    Mount::builder()
        .fstype("")
        .flags(MountFlags::BIND | MountFlags::REC)
        .mount(newroot_pn.clone(), newroot_pn.clone())?;

    env::set_current_dir(newroot_pn.clone())?;

    Mount::builder()
        .fstype("none")
        .flags(MountFlags::BIND | MountFlags::RDONLY)
        .mount("/lib", "lib")?;

    
    Mount::builder()
        .fstype("none")
        .flags(MountFlags::BIND | MountFlags::RDONLY)
        .mount("/lib64", "lib64")?;

    let mut shome : String = sigmahome.to_owned();

    Mount::builder()
        .fstype("proc")
        .mount("proc", "proc")?;

    Mount::builder()
        .fstype("none")
        .flags(MountFlags::BIND | MountFlags::RDONLY)
        .mount(shome+"bin/user", "bin")?;

    shome = sigmahome.to_owned();
    Mount::builder()
        .fstype("none")
        .flags(MountFlags::BIND | MountFlags::RDONLY)
        .mount(shome+"bin/kernel", "bin2")?;

    shome = sigmahome.to_owned();
    Mount::builder()
        .fstype("none")
        .flags(MountFlags::BIND | MountFlags::RDONLY)
        .mount(shome+"seccomp", "seccomp")?;

    Mount::builder()
        .fstype("none")
        .flags(MountFlags::BIND)
        .mount("/cgroup", "cgroup")?;

    // XXX todo: mount perf output
    
    Mount::builder()
        .fstype("none")
        .flags(MountFlags::BIND | MountFlags::RDONLY)
        .mount("/usr", "usr")?;

    Mount::builder()
        .fstype("sysfs")
        .flags(MountFlags::BIND | MountFlags::RDONLY)
        .mount("/sys", "sys")?;

    Mount::builder()
        .fstype("none")
        .flags(MountFlags::BIND | MountFlags::RDONLY)
        .mount("/dev", "dev")?;

    Mount::builder()
        .fstype("none")
        .flags(MountFlags::BIND | MountFlags::RDONLY)
        .mount("/etc", "etc")?;

    pivot_root(".", old_root_mnt)?;

    env::set_current_dir("/")?;

    unmount(old_root_mnt, UnmountFlags::DETACH)?;

    fs::remove_dir(old_root_mnt)?;

    let paths = fs::read_dir("/").unwrap();
    
    for path in paths {
        println!("Name: {}", path.unwrap().path().display())
    }

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
# XXX
  - clone
  - socket

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
    use caps::{CapSet, Capability,CapsHashSet};

    // Taken from https://github.com/moby/moby/blob/master/oci/caps/defaults.go
    let defaults = vec![
	Capability::CAP_CHOWN,
        Capability::CAP_DAC_OVERRIDE,
	Capability::CAP_FSETID,
	Capability::CAP_FOWNER,
	Capability::CAP_NET_RAW,
	Capability::CAP_SETGID,
	Capability::CAP_SETUID,
	Capability::CAP_SETFCAP,
	Capability::CAP_SETPCAP,
	Capability::CAP_NET_BIND_SERVICE,
	Capability::CAP_SYS_CHROOT,
	Capability::CAP_KILL,
	Capability::CAP_AUDIT_WRITE,
    ];

    let defaultsextra = vec![
        Capability::CAP_MKNOD, Capability::CAP_IPC_LOCK, Capability::CAP_NET_BIND_SERVICE, Capability::CAP_SYS_ADMIN, Capability::CAP_PERFMON, Capability::CAP_FSETID, Capability::CAP_SYSLOG, Capability::CAP_SETUID, Capability::CAP_MAC_ADMIN, Capability::CAP_AUDIT_WRITE, Capability::CAP_NET_BROADCAST, Capability::CAP_SYS_RESOURCE, Capability::CAP_SYS_BOOT, Capability::CAP_BPF, Capability::CAP_DAC_OVERRIDE, Capability::CAP_SYS_PTRACE, Capability::CAP_SETFCAP, Capability::CAP_SYS_TIME, Capability::CAP_LEASE, Capability::CAP_SETPCAP, Capability::CAP_LINUX_IMMUTABLE, Capability::CAP_NET_RAW, Capability::CAP_WAKE_ALARM, Capability::CAP_BLOCK_SUSPEND, Capability::CAP_CHECKPOINT_RESTORE, Capability::CAP_SYS_RAWIO, Capability::CAP_SYS_NICE, Capability::CAP_FOWNER, Capability::CAP_KILL, Capability::CAP_SYS_PACCT, Capability::CAP_CHOWN, Capability::CAP_IPC_OWNER, Capability::CAP_AUDIT_READ, Capability::CAP_MAC_OVERRIDE, Capability::CAP_AUDIT_CONTROL, Capability::CAP_SETGID, Capability::CAP_DAC_READ_SEARCH, Capability::CAP_SYS_MODULE, Capability::CAP_SYS_TTY_CONFIG, Capability::CAP_NET_ADMIN, Capability::CAP_SYS_CHROOT,
        ];
    
    let cur = caps::read(None, CapSet::Permitted)?;
    println!("Current permitted caps: {:?}.", cur);
    let cur = caps::read(None, CapSet::Effective)?;
    println!("Current effective caps: {:?}.", cur);
    let cur = caps::read(None, CapSet::Inheritable)?;
    println!("Current inheritable caps: {:?}.", cur);
    let cur = caps::read(None, CapSet::Ambient)?;
    println!("Current ambient caps: {:?}.", cur);

    
    let new_caps = CapsHashSet::from_iter(defaults);
    let new_caps_extra = CapsHashSet::from_iter(defaultsextra);

    println!("new caps: {:?}.", new_caps);

    caps::set(None, CapSet::Permitted, &new_caps_extra)?;

    println!("set permitted caps: {:?}.", new_caps);
    
    caps::set(None, CapSet::Inheritable, &new_caps)?;

    println!("set inherited done: {:?}", new_caps);
    
    let cur = caps::read(None, CapSet::Permitted)?;
    println!("Current permitted caps: {:?}.", cur);

    Ok(())
}
