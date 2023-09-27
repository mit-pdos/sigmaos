use std::env;
use std::process::Command;
use std::os::unix::process::CommandExt;
use std::time::{SystemTime, UNIX_EPOCH};

fn main() {
    let exec_time = env::var("SIGMA_EXEC_TIME").unwrap_or("".to_string());
    let exec_time_micro: u64 = exec_time.parse().unwrap_or(0);

    eprintln!("exec_uproc SIGMA_EXEC_TIME {}", exec_time_micro);

    let pn = env::args().nth(1).expect("no program");
    let new_args: Vec<_> = std::env::args_os().skip(1).collect();
    let mut cmd = Command::new(pn);

    let now = SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .expect("Time went backwards");
     
    env::set_var("SIGMA_EXEC_TIME", now.as_micros().to_string());

    cmd.args(new_args).exec();
}
