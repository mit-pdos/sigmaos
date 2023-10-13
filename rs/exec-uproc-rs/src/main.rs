use chrono::Local;
use env_logger::Builder;
use log::LevelFilter;
use std::env;
use std::fs;
use std::io::Write;
use std::os::unix::process::CommandExt;
use std::process::Command;
use std::time::{Duration, SystemTime, UNIX_EPOCH};

use json;

use serde::{Deserialize, Serialize};
use serde_yaml::{self};

fn print_elapsed_time(msg: &str, start: SystemTime) {
    let elapsed = SystemTime::now()
        .duration_since(start)
        .expect("Time went backwards");
    log::info!("SPAWN_LAT {}: {}us", msg, elapsed.as_micros());
}

fn main() {
    let debug_pid = env::var("SIGMADEBUGPID").unwrap();
    // Set log print formatting to match SigmaOS
    Builder::new()
        .format(move |buf, record| {
            writeln!(
                buf,
                "{} {} {}",
                Local::now().format("%H:%M:%S%.6f"),
                debug_pid,
                record.args()
            )
        })
        .filter(None, LevelFilter::Info)
        .init();

    let exec_time = env::var("SIGMA_EXEC_TIME").unwrap_or("".to_string());
    let exec_time_micros: u64 = exec_time.parse().unwrap_or(0);
    let exec_time = UNIX_EPOCH + Duration::from_micros(exec_time_micros);
    print_elapsed_time("trampoline.exec_trampoline", exec_time);

    let cfg = env::var("SIGMACONFIG").unwrap_or("".to_string());
    let parsed = json::parse(&cfg).unwrap();

    log::info!("Cfg: {}", parsed);

    let program = env::args().nth(1).expect("no program");
    let pid = parsed["pidStr"].as_str().unwrap_or("no pid");
    let mut now = SystemTime::now();
    let aa = is_enabled_apparmor();
    print_elapsed_time("Check apparmor enabled", now);
    now = SystemTime::now();
    jail_proc(pid).expect("jail failed");
    print_elapsed_time("trampoline.fs_jail_proc", now);
    now = SystemTime::now();
    setcap_proc().expect("set caps failed");
    print_elapsed_time("trampoline.setcap_proc", now);
    now = SystemTime::now();
    seccomp_proc().expect("seccomp failed");
    print_elapsed_time("trampoline.seccomp_proc", now);
    if aa {
        now = SystemTime::now();
        apply_apparmor("sigmaos-uproc").expect("apparmor failed");
        print_elapsed_time("trampoline.apply_apparmor", now);
    }

    let new_args: Vec<_> = std::env::args_os().skip(2).collect();
    let mut cmd = Command::new(program.clone());

    // Reset the exec time
    now = SystemTime::now();
    env::set_var(
        "SIGMA_EXEC_TIME",
        now.duration_since(UNIX_EPOCH)
            .expect("Time went backwards")
            .as_micros()
            .to_string(),
    );

    log::info!("exec: {} {:?}", program, new_args);

    let err = cmd.args(new_args).exec();

    log::info!("err: {}", err);
}

fn jail_proc(pid: &str) -> Result<(), Box<dyn std::error::Error>> {
    extern crate sys_mount;
    use nix::unistd::pivot_root;
    use sys_mount::{unmount, Mount, MountFlags, UnmountFlags};

    let old_root_mnt = "oldroot";
    const DIRS: &'static [&'static str] = &[
        "", "oldroot", "lib", "usr", "lib64", "etc", "dev", "cgroup", "proc", "bin", "tmp",
    ];

    let newroot = "/home/sigmaos/jail/";
    let sigmahome = "/home/sigmaos/";
    let newroot_pn: String = newroot.to_owned() + pid + "/";

    // Create directories to use as mount points, as well as the new
    // root directory itself
    for d in DIRS.iter() {
        let path: String = newroot_pn.to_owned();
        fs::create_dir_all(path + d)?;
    }

    log::info!("mount newroot {}", newroot_pn);
    // Mount new file system as a mount point so we can pivot_root to
    // it later
    Mount::builder()
        .fstype("")
        .flags(MountFlags::BIND | MountFlags::REC)
        .mount(newroot_pn.clone(), newroot_pn.clone())?;

    // Chdir to new root
    env::set_current_dir(newroot_pn.clone())?;

    // E.g., openat "/lib/ld-musl-x86_64.so.1"
    Mount::builder()
        .fstype("none")
        .flags(MountFlags::BIND | MountFlags::RDONLY)
        .mount("/lib", "lib")?;

    // Why mount lib64?
    Mount::builder()
        .fstype("none")
        .flags(MountFlags::BIND | MountFlags::RDONLY)
        .mount("/lib64", "lib64")?;

    // A child must be able to stat "/cgroup/cgroup.controllers"
    Mount::builder()
        .fstype("none")
        .flags(MountFlags::BIND | MountFlags::RDONLY)
        .mount("/cgroup", "cgroup")?;

    // E.g., /usr/lib for shared libraries and /usr/local/lib
    Mount::builder()
        .fstype("none")
        .flags(MountFlags::BIND | MountFlags::RDONLY)
        .mount("/usr", "usr")?;

    // E.g., Open "/etc/localtime"
    Mount::builder()
        .fstype("none")
        .flags(MountFlags::BIND | MountFlags::RDONLY)
        .mount("/etc", "etc")?;

    // E.g., Open "/dev/null", "/dev/urandom"
    // Mount::builder()
    //     .fstype("none")
    //     .flags(MountFlags::BIND | MountFlags::RDONLY)
    //     .mount("/dev", "dev")?;

    // E.g., openat "/proc/meminfo", "/proc/self/exe"
    Mount::builder().fstype("proc").mount("proc", "proc")?;

    // To download sigmaos user binaries into
    let shome: String = sigmahome.to_owned();
    Mount::builder()
        .fstype("none")
        .flags(MountFlags::BIND | MountFlags::RDONLY)
        .mount(shome + "bin/user", "bin")?;

    // Only mount /tmp directory if SIGMAPERF is set (meaning we are
    // benchmarking and want to extract the results)
    if env::var("SIGMAPERF").is_ok() {
        // E.g., write pprof files to /tmp/sigmaos-perf
        Mount::builder()
            .fstype("none")
            .flags(MountFlags::BIND)
            .mount("/tmp", "tmp")?;
    }

    // ========== No more mounts beyond this point ==========
    pivot_root(".", old_root_mnt)?;

    env::set_current_dir("/")?;

    unmount(old_root_mnt, UnmountFlags::DETACH)?;

    fs::remove_dir(old_root_mnt)?;

    Ok(())
}

#[derive(Debug, Serialize, Deserialize)]
struct Config {
    allowed: Vec<String>,
    cond_allowed: Vec<Cond>,
}
#[derive(Debug, Serialize, Deserialize)]
struct Cond {
    name: String,
    index: u32,
    op1: u64,
    op: String,
}

fn seccomp_proc() -> Result<(), Box<dyn std::error::Error>> {
    use libseccomp::*;

    let yaml_str = r#"
allowed:
  - accept4
  - access
  - arch_prctl # Enabled by Docker on AMD64, which is the only architecture we're running on at the moment.
  - bind
  - brk
  # - clone3 # Needed by Go runtime on old versions of docker. See https://github.com/moby/moby/issues/42680
  - close
  - connect
  - epoll_create1
  - epoll_ctl
  - epoll_ctl_old
  - epoll_pwait
  - epoll_pwait2
  - execve
  - exit #  if process must stop (e.g., syscall is blocked), it must be able to exit
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
  - open  # to open binary and shared libraries
  - pipe2  # used by go runtime 
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
  - name: socket # Allowed by docker if arg0 != 40 (disallows AF_VSOCK).
    index: 0
    op1: 40
    op: "SCMP_CMP_NE"
  - name: clone
    index: 0
    op1: 0x7E020000
    op: "SCMP_CMP_MASKED_EQ"  # clone flags
"#;

    let cfg: Config = serde_yaml::from_str(&yaml_str)?;
    let mut filter = ScmpFilterContext::new_filter(ScmpAction::Errno(1))?;
    for name in cfg.allowed {
        let syscall = ScmpSyscall::from_name(&name)?;
        filter.add_rule(ScmpAction::Allow, syscall)?;
    }
    for c in cfg.cond_allowed.iter() {
        let syscall = ScmpSyscall::from_name(&c.name)?;
        let cond = ScmpArgCompare::new(c.index, c.op.parse().unwrap(), c.op1);
        filter.add_rule_conditional(ScmpAction::Allow, syscall, &[cond])?;
    }
    filter.load()?;
    Ok(())
}

fn setcap_proc() -> Result<(), Box<dyn std::error::Error>> {
    use caps::{CapSet, Capability};

    // Taken from https://github.com/moby/moby/blob/master/oci/caps/defaults.go
    let _defaults = vec![
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

    // let new_caps = CapsHashSet::from_iter(defaults);
    // log::info!("new caps: {:?}.", new_caps);

    // Must drop caps from Effective before able to drop them from
    // Permitted, but user procs don't need any procs, so just clear.
    caps::clear(None, CapSet::Effective)?;
    // caps::set(None, CapSet::Permitted, &new_caps)?;
    caps::clear(None, CapSet::Permitted)?;
    caps::clear(None, CapSet::Inheritable)?;

    let cur = caps::read(None, CapSet::Permitted)?;
    log::info!("Current permitted caps: {:?}.", cur);

    Ok(())
}

pub fn is_enabled_apparmor() -> bool {
    let apparmor: &str = "/sys/module/apparmor/parameters/enabled";
    let aa_enabled = fs::read_to_string(apparmor);
    match aa_enabled {
        Ok(val) => val.starts_with('Y'),
        Err(_) => false,
    }
}

pub fn apply_apparmor(profile: &str) -> Result<(), Box<dyn std::error::Error>> {
    fs::write("/proc/self/attr/apparmor/exec", format!("exec {profile}"))?;
    Ok(())
}
