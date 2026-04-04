use chrono::Local;
use env_logger::Builder;
use log::LevelFilter;
use nix::fcntl;
use nix::fcntl::FcntlArg;
use nix::fcntl::FdFlag;
use std::env;
use std::fs;
use std::io::Write;
use std::os::fd::IntoRawFd;
use std::os::unix::net::UnixStream;
use std::os::unix::process::CommandExt;
use std::path::Path;
use std::process::Command;
use std::time::{Duration, SystemTime, UNIX_EPOCH};

use serde::{Deserialize, Serialize};

const VERBOSE: bool = true;

fn print_elapsed_time(
    debug_pid: &str,
    msg: &str,
    spawn_time: SystemTime,
    op_start: SystemTime,
    ignore_verbose: bool,
) {
    if ignore_verbose || VERBOSE {
        let op_elapsed = SystemTime::now()
            .duration_since(op_start)
            .expect("Time went backwards");
        let spawn_elapsed = SystemTime::now()
            .duration_since(spawn_time)
            .expect("Time went backwards");
        log::info!(
            "SPAWN_LAT [{}] {} op:{}us sinceSpawn:{}us",
            debug_pid,
            msg,
            op_elapsed.as_micros(),
            spawn_elapsed.as_micros()
        );
    }
}

fn main() {
    let debug_pid = env::var("SIGMADEBUGPID").unwrap();
    let debug_pid_2 = env::var("SIGMADEBUGPID").unwrap();
    // Set log print formatting to match SigmaOS
    Builder::new()
        .format(move |buf, record| {
            writeln!(
                buf,
                "{} {} {}",
                Local::now().format("%H:%M:%S%.6f"),
                debug_pid_2,
                record.args()
            )
        })
        .filter(None, LevelFilter::Info)
        .init();
    let exec_time = env::var("SIGMA_EXEC_TIME").unwrap_or("".to_string());
    let exec_time_micros: u64 = exec_time.parse().unwrap_or(0);
    let exec_time = UNIX_EPOCH + Duration::from_micros(exec_time_micros);
    let spawn_time = env::var("SIGMA_SPAWN_TIME").unwrap_or("".to_string());
    let spawn_time_micros: u64 = spawn_time.parse().unwrap_or(0);
    let spawn_time = UNIX_EPOCH + Duration::from_micros(spawn_time_micros);
    print_elapsed_time(
        &debug_pid,
        "trampoline.exec_trampoline",
        spawn_time,
        exec_time,
        false,
    );
    let pid = env::args().nth(1).expect("no pid");
    let program = env::args().nth(2).expect("no program");
    let dialproxy = env::args().nth(3).expect("no dialproxy");
    let mut now = SystemTime::now();
    let aa = is_enabled_apparmor();
    print_elapsed_time(
        &debug_pid,
        "trampoline.check_apparmor",
        spawn_time,
        now,
        false,
    );
    now = SystemTime::now();
    jail_proc().expect("jail failed");
    print_elapsed_time(
        &debug_pid,
        "trampoline.fs_jail_proc",
        spawn_time,
        now,
        false,
    );
    now = SystemTime::now();
    setcap_proc().expect("set caps failed");
    print_elapsed_time(&debug_pid, "trampoline.setcap_proc", spawn_time, now, false);
    now = SystemTime::now();
    // Get principal ID from env
    let principal_id = env::var("SIGMAPRINCIPAL").unwrap_or("NO_PRINCIPAL_IN_ENV".to_string());
    let principal_id_frame_nbytes = (principal_id.len() + 4) as i32;
    // Connect to the dialproxy socket
    let mut dialproxy_conn = UnixStream::connect("/tmp/spproxyd/spproxyd-dialproxy.sock").unwrap();
    // Write frame containing principal ID to socket
    dialproxy_conn
        .write_all(&i32::to_le_bytes(principal_id_frame_nbytes))
        .unwrap();
    dialproxy_conn.write_all(principal_id.as_bytes()).unwrap();
    // Remove O_CLOEXEC flag so that the connection remains open when the
    // trampoline execs the proc.
    let dialproxy_conn_fd = dialproxy_conn.into_raw_fd();
    fcntl::fcntl(dialproxy_conn_fd, FcntlArg::F_SETFD(FdFlag::empty())).unwrap();
    // Pass the dialproxy socket connection FD to the user proc
    env::set_var("SIGMA_DIALPROXY_FD", dialproxy_conn_fd.to_string());
    print_elapsed_time(
        &debug_pid,
        "trampoline.connect_dialproxy",
        spawn_time,
        now,
        false,
    );
    now = SystemTime::now();
    //    seccomp_proc(spawn_time, dialproxy).expect("seccomp failed");
    print_elapsed_time(
        &debug_pid,
        "trampoline.seccomp_proc",
        spawn_time,
        now,
        false,
    );
    now = SystemTime::now();

    if aa {
        apply_apparmor("sigmaos-uproc").expect("apparmor failed");
        print_elapsed_time(
            &debug_pid,
            "trampoline.apply_apparmor",
            spawn_time,
            now,
            false,
        );

        // TODO: if env::var("SIGMAPERF").is_ok()
        //       apply a AppArmor profile that allows writing to /tmp/sigmaos-perf
        //       otherwise, apply a profile that prohibits this.
    }

    let new_args: Vec<_> = std::env::args_os().skip(4).collect();
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

    print_elapsed_time(&debug_pid, "Setup.Isolation", spawn_time, exec_time, true);
    print_elapsed_time(
        &debug_pid,
        "Paper.Setup.ContainerStart",
        spawn_time,
        exec_time,
        true,
    );
    let err = cmd.args(new_args).exec();
    // Exec should never return
    log::info!("err: {}", err);
    std::process::exit(1);
}


fn jail_proc() -> Result<(), Box<dyn std::error::Error>> {
    use std::fs::File;
    use std::os::unix::io::AsRawFd;
    use nix::sched::setns;
    use nix::sched::CloneFlags;

    // Join the mount namespace
    let ns_file = File::open("/home/sigmaos/jail/mntns")?;
    setns(ns_file, CloneFlags::CLONE_NEWNS)?;

    std::env::set_current_dir("/")?;

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

fn seccomp_proc(
    spawn_time: SystemTime,
    debug_pid: &str,
    dialproxy: String,
) -> Result<(), Box<dyn std::error::Error>> {
    use libseccomp::*;

    // XXX Should really be 64 syscalls. We can remove ioctl, poll, and lstat,
    // but the mini rust proc for our spawn latency microbenchmarks requires
    // it.
    const ALLOWED_SYSCALLS: [ScmpSyscall; 74] = [
        ScmpSyscall::new("ioctl"), // XXX Only needed for rust proc spawn microbenchmark
        ScmpSyscall::new("poll"),  // XXX Only needed for rust proc spawn microbenchmark
        ScmpSyscall::new("lstat"), // XXX Only needed for rust proc spawn microbenchmark
        ScmpSyscall::new("clock_gettime"), // XXX Only needed to run on gVisor
        ScmpSyscall::new("membarrier"), // XXX Only needed to run on gVisor
        ScmpSyscall::new("accept4"),
        ScmpSyscall::new("access"),
        ScmpSyscall::new("arch_prctl"), // Enabled by Docker on AMD64, which is the only architecture we're running on at the moment.
        ScmpSyscall::new("brk"),
        ScmpSyscall::new("close"),
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
        ScmpSyscall::new("lseek"),
        ScmpSyscall::new("madvise"),
        ScmpSyscall::new("mkdirat"),
        ScmpSyscall::new("mmap"),
        ScmpSyscall::new("mremap"),
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
        ScmpSyscall::new("recvmsg"),
        ScmpSyscall::new("restart_syscall"),
        ScmpSyscall::new("rt_sigaction"),
        ScmpSyscall::new("rt_sigprocmask"),
        ScmpSyscall::new("rt_sigreturn"),
        ScmpSyscall::new("sched_getaffinity"),
        ScmpSyscall::new("sched_yield"),
        ScmpSyscall::new("sendto"),
        ScmpSyscall::new("sendmsg"), // Needed for current implementation of dialproxy transport for simplicity of implementation, but not actually needed
        ScmpSyscall::new("setitimer"),
        ScmpSyscall::new("setsockopt"), // Important for performance! (especially hotel/socialnet)
        ScmpSyscall::new("set_robust_list"),
        ScmpSyscall::new("set_tid_address"),
        ScmpSyscall::new("sigaltstack"),
        ScmpSyscall::new("sync"),
        ScmpSyscall::new("timer_create"),
        ScmpSyscall::new("timer_delete"),
        ScmpSyscall::new("timer_settime"),
        ScmpSyscall::new("tgkill"),
        ScmpSyscall::new("write"),
        ScmpSyscall::new("writev"),
        ScmpSyscall::new("readlink"), // Needed for MUSL/Alpine
        ScmpSyscall::new("getcwd"),   // Needed for Python
        ScmpSyscall::new("gettid"),
        ScmpSyscall::new("stat"),
        ScmpSyscall::new("readv"),
        ScmpSyscall::new("uname"), // Numpy
    ];

    const NODIALPROXY_ALLOWED_SYSCALLS: [ScmpSyscall; 3] = [
        ScmpSyscall::new("bind"),
        ScmpSyscall::new("listen"),
        ScmpSyscall::new("connect"),
    ];

    const COND_ALLOWED_SYSCALLS: [(ScmpSyscall, ScmpArgCompare); 1] = [(
        ScmpSyscall::new("clone"),
        ScmpArgCompare::new(0, ScmpCompareOp::MaskedEqual(0), 0x7E020000),
    )];

    const NODIALPROXY_COND_ALLOWED_SYSCALLS: [(ScmpSyscall, ScmpArgCompare); 1] = [(
        ScmpSyscall::new("socket"),
        ScmpArgCompare::new(0, ScmpCompareOp::NotEqual, 40),
    )];

    let mut filter = ScmpFilterContext::new_filter(ScmpAction::Errno(1))?;
    for syscall in ALLOWED_SYSCALLS {
        filter.add_rule(ScmpAction::Allow, syscall)?;
    }
    for c in COND_ALLOWED_SYSCALLS {
        let syscall = c.0;
        let cond = c.1;
        filter.add_rule_conditional(ScmpAction::Allow, syscall, &[cond])?;
    }

    if dialproxy == "false" {
        for syscall in NODIALPROXY_ALLOWED_SYSCALLS {
            filter.add_rule(ScmpAction::Allow, syscall)?;
        }
        for c in NODIALPROXY_COND_ALLOWED_SYSCALLS {
            let syscall = c.0;
            let cond = c.1;
            filter.add_rule_conditional(ScmpAction::Allow, syscall, &[cond])?;
        }
    }
    let now = SystemTime::now();
    filter.load()?;
    print_elapsed_time(
        &debug_pid,
        "trampoline.seccomp_proc load",
        spawn_time,
        now,
        false,
    );
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

pub fn lsdir(pn: &str) {
    println!("lsdir {}", pn);
    let paths = fs::read_dir(pn).unwrap();
    for path in paths {
        println!("Name: {}", path.unwrap().path().display())
    }
}
