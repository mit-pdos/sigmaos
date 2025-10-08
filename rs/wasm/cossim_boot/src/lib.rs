use proto::cache;
use proto::sigmap;
use protobuf::{Message, MessageField};
use sigmaos;
use std::os::raw::c_char;
use std::slice;

const NSHARD: u32 = 1009;

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

fn key2server(key: &String, nserver: u32) -> u32 {
    return key2shard(key) % nserver;
}

#[export_name = "boot"]
pub fn boot(b: *mut c_char, buf_sz: usize) {
    // Convert the shared buffer to a slice
    let buf: &mut [u8] = unsafe { slice::from_raw_parts_mut(b as *mut u8, buf_sz) };
    // Get the input arguments to the boot script
    let n_srv = u32::from_le_bytes(buf[0..4].try_into().unwrap());
    let n_keys = u64::from_le_bytes(buf[4..12].try_into().unwrap());
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
    for rpc_idx in 0..multi_get_rpcs.len() {
        let rpc_bytes = multi_get_rpcs[rpc_idx].write_to_bytes().unwrap();
        let pn = "name/cache/servers/".to_owned() + &rpc_idx.to_string();
        sigmaos::send_rpc(
            buf,
            rpc_idx.try_into().unwrap(),
            &pn,
            "CacheSrv.MultiGet",
            &rpc_bytes,
            2,
        );
    }
}
