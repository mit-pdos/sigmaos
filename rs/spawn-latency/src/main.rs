use chrono::Local;
use std::env;
use std::process;
use std::time::{Duration, SystemTime, UNIX_EPOCH};

const VERBOSE: bool = false;

fn print_elapsed_time(db_pid: &str, msg: &str, start: SystemTime, ignore_verbose: bool) {
    if ignore_verbose || VERBOSE {
        let elapsed = SystemTime::now()
            .duration_since(start)
            .expect("Time went backwards");
        println!(
            "{} {} SPAWN_LAT {}: {}us",
            Local::now().format("%H:%M:%S%.6f"),
            db_pid,
            msg,
            elapsed.as_micros()
        );
    }
}

fn main() {
    let debug_pid = env::var("SIGMADEBUGPID").unwrap();

    let exec_time = env::var("SIGMA_EXEC_TIME").unwrap_or("".to_string());
    let exec_time_micros: u64 = exec_time.parse().unwrap_or(0);
    let exec_time = UNIX_EPOCH + Duration::from_micros(exec_time_micros);

    let spawn_time = env::var("SIGMA_SPAWN_TIME").unwrap_or("".to_string());
    let spawn_time_micros: u64 = spawn_time.parse().unwrap_or(0);
    let spawn_time = UNIX_EPOCH + Duration::from_micros(spawn_time_micros);

    print_elapsed_time(&debug_pid, "proc.exec_proc", exec_time, false);
    print_elapsed_time(&debug_pid, "E2e spawn time since spawn until main", spawn_time, true);

    process::exit(1);
}
