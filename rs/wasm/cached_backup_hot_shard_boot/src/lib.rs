use proto::cache;
use proto::sigmap;
use protobuf::{Message, MessageField};
use sigmaos;
use std::os::raw::c_char;
use std::slice;

//fn zero_buf(buf: &mut [u8], nbyte: usize) {
//    buf[0..nbyte].fill(0);
//}

#[unsafe(export_name = "boot")]
pub fn boot(b: *mut c_char, buf_sz: usize) {
    // Convert the shared buffer to a slice
    let buf: &mut [u8] = unsafe { slice::from_raw_parts_mut(b as *mut u8, buf_sz) };
    let primary_srv_id = u32::from_le_bytes(buf[0..4].try_into().unwrap());
    let top_n = u32::from_le_bytes(buf[4..8].try_into().unwrap());
    let pn = "name/cache/servers/".to_owned() + &primary_srv_id.to_string();
    let mut hot_shards_req = cache::HotShardsReq::new();
    hot_shards_req.topN = top_n;
    let rpc_bytes = hot_shards_req.write_to_bytes().unwrap();
    sigmaos::send_rpc(buf, 0, &pn, "CacheSrv.GetHotShards", &rpc_bytes, 1);
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
    for i in 0..shard_req_rpcs.len() {
        let rpc_idx = i + 1;
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
