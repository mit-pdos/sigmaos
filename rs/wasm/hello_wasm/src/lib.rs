use std::mem;
use std::os::raw::c_char;
use std::slice;

mod sigmaos {
    mod sigmaos_host {
        #[link(wasm_import_module = "sigmaos_host")]
        extern "C" {
            pub fn log_int(i: i32);
        }
    }
    pub fn log_int(i: i32) {
        unsafe {
            sigmaos_host::log_int(i);
        }
    }
}

#[export_name = "allocate"]
pub fn allocate(size: usize) -> *mut c_char {
    let mut buffer = Vec::with_capacity(size);
    let pointer = buffer.as_mut_ptr();
    mem::forget(buffer);
    pointer as *mut c_char
}

#[export_name = "add"]
pub fn add(left: usize, right: usize) -> usize {
    left + right
}

#[export_name = "add_and_log"]
pub fn add_and_log(left: usize, right: usize) -> usize {
    let n: i32 = 1234543;
    sigmaos::log_int(n);
    left + right
}

#[export_name = "add_and_log_with_mem"]
pub fn add_and_log_with_mem(left: usize, right: usize, b: *mut c_char, len: usize) -> usize {
    let buf_slice: &mut [i8] = unsafe { slice::from_raw_parts_mut(b, len) };
    let n: i32 = buf_slice[0].into();
    buf_slice[1] = (n + 1) as i8;
    sigmaos::log_int(n);
    left + right
}
