use proto::cache;
use proto::sigmap;
use protobuf::{Message, MessageField};
use sigmaos;
use std::os::raw::c_char;
use std::slice;

const NSHARD: u32 = 1009;

fn zero_buf(buf: &mut [u8], nbyte: usize) {
    buf[0..nbyte].fill(0);
}

#[unsafe(export_name = "boot")]
pub fn boot(b: *mut c_char, buf_sz: usize) {
    // Convert the shared buffer to a slice
    let buf: &mut [u8] = unsafe { slice::from_raw_parts_mut(b as *mut u8, buf_sz) };
    let primary_srv_id = u32::from_le_bytes(buf[0..4].try_into().unwrap());
    let top_n = u32::from_le_bytes(buf[4..8].try_into().unwrap());
    let pn = "name/cache/servers/".to_owned() + &primary_srv_id.to_string();
    let mut hot_shards_req = cache::HotShardsReq::new();
    hot_shards_req.topN = top_n;
    let v = hot_shards_req.write_to_bytes().unwrap();
    // Make the GetHotShards RPC
    let mut idx = 0;
    let pn_len = pn.len();
    for c in pn.bytes() {
        buf[idx] = c;
        idx += 1;
    }
    let mut method_len = 0;
    for c in "CacheSrv.GetHotShards".bytes() {
        buf[idx] = c;
        idx += 1;
        method_len += 1;
    }
    for b in &v {
        buf[idx] = *b;
        idx += 1;
    }
    let mut write_sz: usize = 0;
    // Record the serialized protobuf size, so we can zero the buffer again
    // before writing the next protobuf
    write_sz = pn_len as usize + method_len as usize + v.len() as usize;
    sigmaos::send_rpc(0, pn_len as u64, method_len as u64, v.len() as u64, 1);
    // Await the reply
    let rep_nbyte = sigmaos::recv_rpc(0) as usize;
    // Resize the buffer
    let hot_shards_rep = cache::HotShardsRep::parse_from_bytes(&buf[0..rep_nbyte]).unwrap();
    // TODO: check hot shards len isn't 0
    // Create a shard request for each shard
    let mut shard_req_rpcs: Vec<cache::ShardReq> = Vec::new();
    for idx in 0..hot_shards_rep.shardIDs.len() {
        let mut shard_req = cache::ShardReq::new();
        shard_req.shard = hot_shards_rep.shardIDs[idx];
        shard_req.fence = MessageField::some(sigmap::TfenceProto::new());
        shard_req_rpcs.push(shard_req);
    }
    // Initial buffer contents are the 4-byte n_srv and the 8-byte n_keys
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
