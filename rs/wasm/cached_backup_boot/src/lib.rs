use proto::cache;
use proto::sigmap;
use protobuf::{Message, MessageField};
use sigmaos;
use std::mem;
use std::os::raw::c_char;
use std::slice;

const NSHARD: u32 = 1009;

#[unsafe(export_name = "allocate")]
pub fn allocate(size: usize) -> *mut c_char {
    let mut buffer = Vec::with_capacity(size);
    let pointer = buffer.as_mut_ptr();
    mem::forget(buffer);
    pointer as *mut c_char
}

fn zero_buf(buf: &mut [u8], nbyte: usize) {
    buf[0..nbyte].fill(0);
}

#[unsafe(export_name = "boot")]
pub fn boot(b: *mut c_char, buf_sz: usize) {
    // Convert the shared buffer to a slice
    let buf: &mut [u8] = unsafe { slice::from_raw_parts_mut(b as *mut u8, buf_sz) };
    let primary_srv_id = u32::from_le_bytes(buf[0..4].try_into().unwrap());
    // Create a shard request for each shard
    let mut shard_req_rpcs: Vec<cache::ShardReq> = Vec::new();
    for srv_id in 0..NSHARD {
        let mut shard_req = cache::ShardReq::new();
        shard_req.shard = srv_id as u32;
        shard_req.fence = MessageField::some(sigmap::TfenceProto::new());
        shard_req_rpcs.push(shard_req);
    }
    // Initial buffer contents are the 4-byte n_srv and the 8-byte n_keys
    let mut write_sz: usize = 0;
    let pn = "name/cache/servers/".to_owned() + &primary_srv_id.to_string();
    for rpc_idx in 0..shard_req_rpcs.len() {
        // First, fill any portion of the buffer previously written to with
        // zeros
        zero_buf(buf, write_sz);
        let v = shard_req_rpcs[rpc_idx].write_to_bytes().unwrap();
        let mut idx = 0;
        let pn_len = pn.len();
        for c in pn.bytes() {
            buf[idx] = c;
            idx += 1;
        }
        let mut method_len = 0;
        for c in "CacheSrv.DumpShard".bytes() {
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
        write_sz = pn_len as usize + method_len as usize + v.len() as usize;
        sigmaos::send_rpc(
            rpc_idx.try_into().unwrap(),
            pn_len as u64,
            method_len as u64,
            v.len() as u64,
            1,
        );
    }
}
