use proto::cache;
use proto::sigmap;
use protobuf::{Message, MessageField};
use sigmaos;
use std::os::raw::c_char;
use std::slice;

const NSHARD: u32 = 1009;

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
    let pn = "name/cache/servers/".to_owned() + &primary_srv_id.to_string();
    for rpc_idx in 0..shard_req_rpcs.len() {
        let rpc_bytes = shard_req_rpcs[rpc_idx].write_to_bytes().unwrap();
        sigmaos::send_rpc(
            buf,
            rpc_idx.try_into().unwrap(),
            &pn,
            "CacheSrv.DumpShard",
            &rpc_bytes,
            1,
        );
    }
}
