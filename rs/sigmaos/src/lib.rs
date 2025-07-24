mod sigmaos_host {
    #[link(wasm_import_module = "sigmaos_host")]
    unsafe extern "C" {
        pub fn send_rpc(rpc_idx: u64, pn_len: u64, method_len: u64, rpc_len: u64, n_outiov: u64);
        pub fn recv_rpc(rpc_idx: u64) -> u64;
    }
}

pub fn send_rpc(rpc_idx: u64, pn_len: u64, method_len: u64, rpc_len: u64, n_outiov: u64) {
    unsafe {
        sigmaos_host::send_rpc(rpc_idx, pn_len, method_len, rpc_len, n_outiov);
    }
}

pub fn recv_rpc(rpc_idx: u64) -> u64 {
    return unsafe { sigmaos_host::recv_rpc(rpc_idx) };
}

//#[unsafe(export_name = "allocate")]
//pub fn allocate(size: usize) -> *mut c_char {
//    let mut buffer = Vec::with_capacity(size);
//    let pointer = buffer.as_mut_ptr();
//    mem::forget(buffer);
//    pointer as *mut c_char
//}
