use proto::cache;
use proto::sigmap;
use protobuf::{Message, MessageField};
use sigmaos;
use std::os::raw::c_char;
use std::slice;

#[unsafe(export_name = "boot")]
pub fn boot(b: *mut c_char, buf_sz: usize) {
    // Convert the shared buffer to a slice
    let buf: &mut [u8] = unsafe { slice::from_raw_parts_mut(b as *mut u8, buf_sz) };
    let srv_id = u32::from_le_bytes(buf[0..4].try_into().unwrap());
    let n_srv = u32::from_le_bytes(buf[4..8].try_into().unwrap());
    let n_shard = u32::from_le_bytes(buf[8..12].try_into().unwrap());
    let mut shards: Vec<u32> = Vec::new();
    for shard in 0..n_shard {
        // If this shard will be assigned to this server in the new assignment
        if shard % n_srv == srv_id {
            shards.push(shard);
        }
    }
    let mut rpc_idx = 0;
    let pn = "name/cache/servers/".to_owned() + &srv_id.to_string();
    let mut multi_shard_req = cache::MultiShardReq::new();
    multi_shard_req.fence = MessageField::some(sigmap::TfenceProto::new());
    for shard in &shards {
        multi_shard_req.shards.push(*shard);
    }
    let rpc_bytes = multi_shard_req.write_to_bytes().unwrap();
    sigmaos::send_rpc(
        buf,
        rpc_idx.try_into().unwrap(),
        &pn,
        "CacheSrv.MultiDumpShard",
        &rpc_bytes,
        (1 + shards.len()).try_into().unwrap(),
    );
}
