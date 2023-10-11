use chrono::Local;
use env_logger::Builder;
use log::LevelFilter;
use std::env;
use std::io::Write;
use std::process;
use std::time::{Duration, SystemTime, UNIX_EPOCH};

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
    print_elapsed_time("proc.exec_proc", exec_time);
    process::exit(1);
}
