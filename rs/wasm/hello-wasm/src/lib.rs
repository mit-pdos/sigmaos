#[link(wasm_import_module = "sigmaos_host")]
extern "C" {
    pub fn log(i: i32);
}

#[no_mangle]
pub fn add(left: usize, right: usize) -> usize {
    left + right
}

#[no_mangle]
pub fn add_and_log(left: usize, right: usize) -> usize {
    let n: i32 = 1234543;
    unsafe {
        log(n);
    }
    left + right
}
