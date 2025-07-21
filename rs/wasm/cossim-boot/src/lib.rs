use protobuf::{Message, MessageField};
use std::mem;
use std::os::raw::c_char;
use std::slice;

mod cache;
mod rpc;
mod sigmap;
mod tracing;

mod sigmaos {
    mod sigmaos_host {
        #[link(wasm_import_module = "sigmaos_host")]
        extern "C" {
            pub fn rpc(idx: u64, len: u64);
        }
    }
    pub fn rpc(idx: u64, len: u64) {
        unsafe {
            sigmaos_host::rpc(idx, len);
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

#[export_name = "boot"]
pub fn boot(key: u64, shard: u32, b: *mut c_char, buf_sz: usize) {
    let buf: &mut [u8] = unsafe { slice::from_raw_parts_mut(b as *mut u8, buf_sz) };
    let n: i32 = buf[0].into();
    let mut multi_get = cache::CacheMultiGetReq::new();
    let fence = sigmap::TfenceProto::new();
    let mut get_descriptor = cache::CacheGetDescriptor::new();
    get_descriptor.key = key.to_string();
    get_descriptor.shard = shard;
    multi_get.fence = MessageField::some(fence);
    multi_get.gets.push(get_descriptor);
    multi_get.write_to_vec(&mut buf.to_vec()).unwrap();
    let v = multi_get.write_to_bytes().unwrap();
    let mut idx = 0;
    for b in &v {
        buf[idx] = *b;
        idx += 1;
    }
    sigmaos::rpc(0, v.len() as u64);
}
