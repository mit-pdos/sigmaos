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

// CPU time (user + system, in microseconds) this process has consumed so far.
fn self_cpu_us() -> i64 {
    match nix::sys::resource::getrusage(nix::sys::resource::UsageWho::RUSAGE_SELF) {
        Ok(ru) => {
            let ut = ru.user_time();
            let st = ru.system_time();
            (ut.tv_sec() as i64 * 1_000_000 + ut.tv_usec() as i64)
                + (st.tv_sec() as i64 * 1_000_000 + st.tv_usec() as i64)
        }
        Err(_) => 0,
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
    jail_proc(spawn_time, &debug_pid, &pid).expect("jail failed");
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

    // Hand over how much CPU this process has burned so far. execve keeps the
    // process's CPU counters (only fork resets them), so the exec'd program's
    // own getrusage would otherwise include all of the trampoline's work; it
    // subtracts this to get the CPU cost of exec -> main alone.
    env::set_var("SIGMA_EXEC_CPU_US", self_cpu_us().to_string());

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

// Jail the proc in the node's shared jail directory. Procd creates it once
// (scontainer.EnsureJail) with the directories and the read-only mounts every
// proc needs, because they are views of the container's own /lib, /usr, /etc,
// ... and so are identical for every proc; building them per proc cost ~16
// mkdirs and ~10 mounts. What is left here is what cannot be shared: /proc,
// since each proc has its own PID namespace, and the mounts which depend on
// this proc's environment. Each proc has its own mount namespace (CLONE_NEWNS
// in StartSigmaContainer), so these mounts and the pivot_root below are
// private to it.
fn jail_proc(
    spawn_time: SystemTime,
    debug_pid: &str,
    pid: &str,
) -> Result<(), Box<dyn std::error::Error>> {
    let mut now = SystemTime::now();
    extern crate sys_mount;
    use nix::unistd::pivot_root;
    use sys_mount::{Mount, MountFlags, UnmountFlags, unmount};

    let old_root_mnt = "oldroot";

    // Must match scontainer.JailPath.
    let newroot_pn = "/home/sigmaos/jail/uproc/";

    if VERBOSE {
        log::info!("jail {} in {}", pid, newroot_pn);
    }

    // Chdir to new root
    env::set_current_dir(newroot_pn)?;

    // E.g., openat "/proc/meminfo", "/proc/self/exe", but further
    // restricted by apparmor sigmoas-uproc profile. Mounted per proc rather
    // than shared: this proc has its own PID namespace, and procd's proc
    // instance would show it the outer namespace's processes.
    Mount::builder().fstype("proc").mount("proc", "proc")?;

    // Only mount /tmp/sigmaos-perf directory if SIGMAPERF is set (meaning we are
    // benchmarking and want to extract the results), so it stays per proc. Note
    // that it is writable, unlike the /tmp it is mounted over.
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
    // Python procs need the sigmaos Python library, the extracted SeBS bundle,
    // and the systemd-resolved directory so that /etc/resolv.conf (which is
    // typically a symlink to /run/systemd/resolve/stub-resolv.conf) can be
    // followed inside the jail for DNS resolution via getaddrinfo(). These stay
    // per proc: bin/user in particular has to be bound after procd has mounted
    // the realm's binaries over /home/sigmaos/bin/user, which happens when it
    // is assigned to a realm rather than when it creates the jail. The
    // dev/urandom and dev/null mount points are files, which procd creates.
    if env::var("SIGMA_PYTHON_PROC").is_ok() {
        Mount::builder()
            .fstype("none")
            .flags(MountFlags::BIND | MountFlags::RDONLY)
            .mount("/home/sigmaos/python", "home/sigmaos/python")?;
        Mount::builder()
            .fstype("none")
            .flags(MountFlags::BIND | MountFlags::RDONLY)
            .mount("/home/sigmaos/bin/user", "bin/user")?;
        Mount::builder()
            .fstype("none")
            .flags(MountFlags::BIND | MountFlags::RDONLY)
            .mount("/run", "run")?;
        Mount::builder()
            .fstype("none")
            .flags(MountFlags::BIND | MountFlags::RDONLY)
            .mount("/dev/urandom", "dev/urandom")?;

        Mount::builder()
            .fstype("none")
            .flags(MountFlags::BIND | MountFlags::RDONLY)
            .mount("/dev/null", "dev/null")?;
    }
    print_elapsed_time(
        &debug_pid,
        "trampoline.fs_jail_proc mount dirs",
        spawn_time,
        now,
        false,
    );
    now = SystemTime::now();
    // ========== No more mounts beyond this point ==========
    pivot_root(".", old_root_mnt)?;
    print_elapsed_time(
        &debug_pid,
        "trampoline.fs_jail_proc pivot_root",
        spawn_time,
        now,
        false,
    );
    now = SystemTime::now();

    env::set_current_dir("/")?;
    print_elapsed_time(
        &debug_pid,
        "trampoline.fs_jail_proc chdir",
        spawn_time,
        now,
        false,
    );
    now = SystemTime::now();

    unmount(old_root_mnt, UnmountFlags::DETACH)?;
    print_elapsed_time(
        &debug_pid,
        "trampoline.fs_jail_proc umount",
        spawn_time,
        now,
        false,
    );

    // Leave the oldroot mount point in place: every proc pivot_roots through
    // it, so it belongs to the shared jail, not to this proc.

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
    const ALLOWED_SYSCALLS: [ScmpSyscall; 69] = [
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
