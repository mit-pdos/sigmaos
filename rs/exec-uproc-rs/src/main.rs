use chrono::Local;
use env_logger::Builder;
use log::LevelFilter;
use std::env;
use std::fs;
use std::io::Write;
use std::os::unix::process::CommandExt;
use std::process::Command;
use std::time::{Duration, SystemTime, UNIX_EPOCH};

use serde::{Deserialize, Serialize};

const VERBOSE: bool = false;

fn print_elapsed_time(/*label: &str,*/ msg: &str, start: SystemTime, ignore_verbose: bool) {
    if ignore_verbose || VERBOSE {
        let elapsed = SystemTime::now()
            .duration_since(start)
            .expect("Time went backwards");
        log::info!("SPAWN_LAT {}: {}us", msg, elapsed.as_micros());
    }
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
    print_elapsed_time("trampoline.exec_trampoline", exec_time, false);

    let pid = env::args().nth(1).expect("no pid");
    let program = env::args().nth(2).expect("no program");
    let mut now = SystemTime::now();
    let aa = is_enabled_apparmor();
    print_elapsed_time("Check apparmor enabled", now, false);
    now = SystemTime::now();
    jail_proc(&pid).expect("jail failed");
    print_elapsed_time("trampoline.fs_jail_proc", now, false);
    now = SystemTime::now();
    setcap_proc().expect("set caps failed");
    print_elapsed_time("trampoline.setcap_proc", now, false);
    now = SystemTime::now();
    seccomp_proc().expect("seccomp failed");
    print_elapsed_time("trampoline.seccomp_proc", now, false);
    now = SystemTime::now();
    if aa {
        apply_apparmor("sigmaos-uproc").expect("apparmor failed");
        print_elapsed_time("trampoline.apply_apparmor", now, false);
    }

    let new_args: Vec<_> = std::env::args_os().skip(3).collect();
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

    if VERBOSE {
        log::info!("exec: {} {:?}", program, new_args);
    }

    let err = cmd.args(new_args).exec();

    log::info!("err: {}", err);
}

fn jail_proc(pid: &str) -> Result<(), Box<dyn std::error::Error>> {
    let mut now = SystemTime::now();
    extern crate sys_mount;
    use nix::unistd::pivot_root;
    use sys_mount::{unmount, Mount, MountFlags, UnmountFlags};

    let old_root_mnt = "oldroot";
    const DIRS: &'static [&'static str] = &[
        "",
        "oldroot",
        "lib",
        "usr",
        "lib64",
        "etc",
        "proc",
        "bin",
        "tmp",
        "dev",
        "tmp/sigmaos-perf",
    ];

    let newroot = "/home/sigmaos/jail/";
    let sigmahome = "/home/sigmaos/";
    let newroot_pn: String = newroot.to_owned() + pid + "/";

    log::info!("using perf: {}", env::var("SIGMAPERF").is_ok());

    // Create directories to use as mount points, as well as the new
    // root directory itself
    for d in DIRS.iter() {
        let path: String = newroot_pn.to_owned();
        fs::create_dir_all(path + d)?;
    }
    print_elapsed_time("trampoline.fs_jail_proc create_dir_all", now, false);
    now = SystemTime::now();

    if VERBOSE {
        log::info!("mount newroot {}", newroot_pn);
    }
    // Mount new file system as a mount point so we can pivot_root to
    // it later
    Mount::builder()
        .fstype("")
        .flags(MountFlags::BIND | MountFlags::REC)
        .mount(newroot_pn.clone(), newroot_pn.clone())?;

    // Chdir to new root
    env::set_current_dir(newroot_pn.clone())?;

    // E.g., execve /lib/ld-musl-x86_64.so.1
    Mount::builder()
        .fstype("none")
        .flags(MountFlags::BIND | MountFlags::RDONLY)
        .mount("/lib", "lib")?;

    // E.g., openat "/lib64/ld-musl-x86_64.so.1" (links to /lib/)
    Mount::builder()
        .fstype("none")
        .flags(MountFlags::BIND | MountFlags::RDONLY)
        .mount("/lib64", "lib64")?;

    // E.g., /usr/lib for shared libraries (e.g., /usr/lib/libseccomp.so.2)
    Mount::builder()
        .fstype("none")
        .flags(MountFlags::BIND | MountFlags::RDONLY)
        .mount("/usr", "usr")?;

    // E.g., Open "/etc/localtime"
    Mount::builder()
        .fstype("none")
        .flags(MountFlags::BIND | MountFlags::RDONLY)
        .mount("/etc", "etc")?;
    
    Mount::builder()
        .fstype("none")
        .flags(MountFlags::BIND | MountFlags::RDONLY)
        .mount("/dev", "dev")?;

    // E.g., openat "/proc/meminfo", "/proc/self/exe", but further
    // restricted by apparmor sigmoas-uproc profile.
    Mount::builder().fstype("proc").mount("proc", "proc")?;

    // To download sigmaos user binaries into /home/sigmaos/bin/user
    let shome: String = sigmahome.to_owned();
    Mount::builder()
        .fstype("none")
        .flags(MountFlags::BIND | MountFlags::RDONLY)
        .mount(shome + "bin/user", "bin")?;

    // Only mount /tmp/sigmaos-perf directory if SIGMAPERF is set (meaning we are
    // benchmarking and want to extract the results)
    if env::var("SIGMAPERF").is_ok() {
        // E.g., write pprof files to /tmp/sigmaos-perf
        Mount::builder()
            .fstype("none")
            .flags(MountFlags::BIND)
            .mount("/tmp/sigmaos-perf", "tmp/sigmaos-perf")?;
        //            .mount("/tmp/sigmaos-perf", "tmp/sigmaos-perf")?;
        if VERBOSE {
            log::info!("PERF {}", "mounting perf dir");
        }
    }
    print_elapsed_time("trampoline.fs_jail_proc mount dirs", now, false);
    now = SystemTime::now();
    // ========== No more mounts beyond this point ==========
    pivot_root(".", old_root_mnt)?;
    print_elapsed_time("trampoline.fs_jail_proc pivot_root", now, false);
    now = SystemTime::now();

    env::set_current_dir("/")?;
    print_elapsed_time("trampoline.fs_jail_proc chdir", now, false);
    now = SystemTime::now();

    unmount(old_root_mnt, UnmountFlags::DETACH)?;
    print_elapsed_time("trampoline.fs_jail_proc umount", now, false);
    now = SystemTime::now();

    fs::remove_dir(old_root_mnt)?;
    print_elapsed_time("trampoline.fs_jail_proc rmdir", now, false);

    Ok(())
}

#[derive(Debug, Serialize, Deserialize)]
struct Config {
    //cond_allowed: Vec<Cond>,
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

    // XXX Should really be 64 syscalls. We can remove ioctl, poll, and lstat,
    // but the mini rust proc for our spawn latency microbenchmarks requires
    // it.
    const ALLOWED_SYSCALLS: [ScmpSyscall; 69] = [
        //const ALLOWED_SYSCALLS: [ScmpSyscall; 67] = [
        ScmpSyscall::new("ioctl"), // XXX Only needed for rust proc spawn microbenchmark
        ScmpSyscall::new("poll"),  // XXX Only needed for rust proc spawn microbenchmark
        ScmpSyscall::new("lstat"), // XXX Only needed for rust proc spawn microbenchmark
        ScmpSyscall::new("clock_gettime"), // XXX Only needed to run on gVisor
        ScmpSyscall::new("membarrier"), // XXX Only needed to run on gVisor
        ScmpSyscall::new("accept4"),
        ScmpSyscall::new("access"),
        ScmpSyscall::new("arch_prctl"), // Enabled by Docker on AMD64, which is the only architecture we're running on at the moment.
        ScmpSyscall::new("bind"),
        ScmpSyscall::new("brk"),
        ScmpSyscall::new("close"),
        ScmpSyscall::new("connect"),
        ScmpSyscall::new("epoll_create1"),
        ScmpSyscall::new("epoll_ctl"),
        ScmpSyscall::new("epoll_ctl_old"),
        ScmpSyscall::new("epoll_pwait"),
        ScmpSyscall::new("epoll_pwait2"),
        ScmpSyscall::new("execve"),
        ScmpSyscall::new("exit"), // if process must stop (e.g., syscall is blocked), it must be able to exit
        ScmpSyscall::new("exit_group"),
        ScmpSyscall::new("fcntl"),
        ScmpSyscall::new("fstat"),
        ScmpSyscall::new("fsync"),
        ScmpSyscall::new("futex"),
        ScmpSyscall::new("getdents64"),
        ScmpSyscall::new("getpeername"),
        ScmpSyscall::new("getpid"),
        ScmpSyscall::new("getrandom"),
        ScmpSyscall::new("getrlimit"),
        ScmpSyscall::new("getsockname"),
        ScmpSyscall::new("getsockopt"),
        ScmpSyscall::new("gettid"),
        ScmpSyscall::new("listen"),
        ScmpSyscall::new("lseek"),
        ScmpSyscall::new("madvise"),
        ScmpSyscall::new("mkdirat"),
        ScmpSyscall::new("mmap"),
        ScmpSyscall::new("mprotect"),
        ScmpSyscall::new("munmap"),
        ScmpSyscall::new("nanosleep"),
        ScmpSyscall::new("newfstatat"),
        ScmpSyscall::new("openat"),
        ScmpSyscall::new("open"),  // to open binary and shared libraries
        ScmpSyscall::new("pipe2"), // used by go runtime
        ScmpSyscall::new("pread64"),
        ScmpSyscall::new("prlimit64"),
        ScmpSyscall::new("read"),
        ScmpSyscall::new("readlinkat"),
        ScmpSyscall::new("recvfrom"),
        ScmpSyscall::new("restart_syscall"),
        ScmpSyscall::new("rt_sigaction"),
        ScmpSyscall::new("rt_sigprocmask"),
        ScmpSyscall::new("rt_sigreturn"),
        ScmpSyscall::new("sched_getaffinity"),
        ScmpSyscall::new("sched_yield"),
        ScmpSyscall::new("sendto"),
        ScmpSyscall::new("setitimer"),
        ScmpSyscall::new("set_robust_list"),
        ScmpSyscall::new("set_tid_address"),
        ScmpSyscall::new("setsockopt"),
        ScmpSyscall::new("sigaltstack"),
        ScmpSyscall::new("sync"),
        ScmpSyscall::new("timer_create"),
        ScmpSyscall::new("timer_delete"),
        ScmpSyscall::new("timer_settime"),
        ScmpSyscall::new("tgkill"),
        ScmpSyscall::new("write"),
        ScmpSyscall::new("writev"),
        ScmpSyscall::new("readlink"), // Needed for MUSL/Alpine
    ];

    const COND_ALLOWED_SYSCALLS: [(ScmpSyscall, ScmpArgCompare); 2] = [
        (
            ScmpSyscall::new("clone"),
            ScmpArgCompare::new(0, ScmpCompareOp::MaskedEqual(0), 0x7E020000),
        ),
        (
            ScmpSyscall::new("socket"),
            ScmpArgCompare::new(0, ScmpCompareOp::NotEqual, 40),
        ),
    ];

    let mut filter = ScmpFilterContext::new_filter(ScmpAction::Errno(1))?;
    for syscall in ALLOWED_SYSCALLS {
        filter.add_rule(ScmpAction::Allow, syscall)?;
    }
    for c in COND_ALLOWED_SYSCALLS {
        let syscall = c.0;
        let cond = c.1;
        filter.add_rule_conditional(ScmpAction::Allow, syscall, &[cond])?;
    }
    let now = SystemTime::now();
    filter.load()?;
    print_elapsed_time("trampoline.seccomp_proc load", now, false);
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
    if VERBOSE {
        log::info!("Current permitted caps: {:?}.", cur);
    }

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
