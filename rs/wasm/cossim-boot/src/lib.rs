use protobuf::{Message, MessageField};
use std::mem;
use std::os::raw::c_char;
use std::slice;

mod cache;
mod rpc;
mod sigmap;
mod tracing;

const NSHARD: u32 = 1009;

mod sigmaos {
    mod sigmaos_host {
        #[link(wasm_import_module = "sigmaos_host")]
        extern "C" {
            pub fn send_rpc(
                rpc_idx: u64,
                pn_len: u64,
                method_len: u64,
                rpc_len: u64,
                n_outiov: u64,
            );
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
}

#[export_name = "allocate"]
pub fn allocate(size: usize) -> *mut c_char {
    let mut buffer = Vec::with_capacity(size);
    let pointer = buffer.as_mut_ptr();
    mem::forget(buffer);
    pointer as *mut c_char
}

fn key2shard(key: &String) -> u32 {
    // fnv32a hash inspired by https://cs.opensource.google/go/go/+/refs/tags/go1.24.3:src/hash/fnv/fnv.go;l=51
    let mut s: u32 = 2166136261;
    let prime32: u32 = 16777619;
    for c in key.bytes() {
        s ^= c as u32;
        s *= prime32;
    }
    return s % NSHARD;
}

fn zero_buf(buf: &mut [u8], nbyte: usize) {
    buf[0..nbyte].fill(0);
}

fn key2server(key: &String, nserver: u32) -> u32 {
    // fnv32a hash inspired by https://cs.opensource.google/go/go/+/refs/tags/go1.24.3:src/hash/fnv/fnv.go;l=51
    let mut s: u32 = 2166136261;
    let prime32: u32 = 16777619;
    for c in key.bytes() {
        s ^= c as u32;
        s *= prime32;
    }
    return s % nserver;
}

#[export_name = "boot"]
pub fn boot(b: *mut c_char, buf_sz: usize) {
    // Convert the shared buffer to a slice
    let buf: &mut [u8] = unsafe { slice::from_raw_parts_mut(b as *mut u8, buf_sz) };
    // Get the input arguments to the boot script
    let n_srv = u32::from_le_bytes(buf[0..4].try_into().unwrap());
    let n_keys = u64::from_le_bytes(buf[4..12].try_into().unwrap());
    // Now zero the slice
    // Create a multi_get request for each server
    let mut multi_get_rpcs: Vec<cache::CacheMultiGetReq> = Vec::new();
    for srv_id in 0..n_srv {
        multi_get_rpcs.push(cache::CacheMultiGetReq::new());
        multi_get_rpcs[srv_id as usize].fence = MessageField::some(sigmap::TfenceProto::new());
    }
    for key in 0..n_keys {
        let mut get_descriptor = cache::CacheGetDescriptor::new();
        get_descriptor.key = key.to_string();
        get_descriptor.shard = key2shard(&get_descriptor.key);
        let srv_id = key2server(&get_descriptor.key, n_srv);
        multi_get_rpcs[srv_id as usize].gets.push(get_descriptor);
    }
    // Initial buffer contents are the 4-byte n_srv and the 8-byte n_keys
    let mut write_sz: usize = 12;
    for rpc_idx in 0..multi_get_rpcs.len() {
        // First, fill any portion of the buffer previously written to with
        // zeros
        zero_buf(buf, write_sz);
        let v = multi_get_rpcs[rpc_idx].write_to_bytes().unwrap();
        let pn_base = "name/cache/servers/".to_owned();
        let mut idx = 0;
        let mut pn_len = 0;
        for c in pn_base.bytes() {
            buf[idx] = c;
            idx += 1;
            pn_len += 1;
        }
        let srv_id = rpc_idx.to_string();
        for c in srv_id.bytes() {
            buf[idx] = c;
            idx += 1;
            pn_len += 1;
        }
        let mut method_len = 0;
        for c in "CacheSrv.MultiGet".bytes() {
            buf[idx] = c;
            idx += 1;
            method_len += 1;
        }
        for b in &v {
            buf[idx] = *b;
            idx += 1;
        }
        // Record the serialized protobuf size, so we can zero the buffer again
        // before writing the next protobuf
        write_sz = pn_len as usize + v.len() as usize;
        sigmaos::send_rpc(
            rpc_idx.try_into().unwrap(),
            pn_len as u64,
            method_len as u64,
            v.len() as u64,
            1,
        );
    }
}
