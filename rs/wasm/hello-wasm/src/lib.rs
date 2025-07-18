mod sigmaos {
    mod sigmaos_host {
        #[link(wasm_import_module = "sigmaos_host")]
        extern "C" {
            pub fn log(i: i32);
        }
    }

    pub fn log(i: i32) {
        unsafe {
            sigmaos_host::log(i);
        }
    }
}

#[export_name = "add"]
pub fn add(left: usize, right: usize) -> usize {
    left + right
}

#[export_name = "add_and_log"]
pub fn add_and_log(left: usize, right: usize) -> usize {
    let n: i32 = 1234543;
    sigmaos::log(n);
    left + right
}
