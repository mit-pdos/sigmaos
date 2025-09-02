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
    let old_n_srv = u32::from_le_bytes(buf[4..8].try_into().unwrap());
    let new_n_srv = u32::from_le_bytes(buf[8..12].try_into().unwrap());
    let n_shard = u32::from_le_bytes(buf[12..16].try_into().unwrap());
    let mut src_srvs: Vec<Vec<u32>> = Vec::new();
    for _ in 0..old_n_srv {
        src_srvs.push(Vec::new());
    }
    for shard in 0..n_shard {
        // If this shard will be assigned to this server in the new assignment
        if shard % new_n_srv == srv_id {
            let src_srv_id = (shard % old_n_srv) as usize;
            src_srvs[src_srv_id].push(shard);
        }
    }
    let mut rpc_idx = 0;
    for (src_srv, shards) in src_srvs.iter().enumerate() {
        let pn = "name/cache/servers/".to_owned() + &src_srv.to_string();
        for shard in shards {
            let mut shard_req = cache::ShardReq::new();
            shard_req.shard = *shard;
            shard_req.fence = MessageField::some(sigmap::TfenceProto::new());
            shard_req.empty = false;
            let rpc_bytes = shard_req.write_to_bytes().unwrap();
            sigmaos::send_rpc(
                buf,
                rpc_idx.try_into().unwrap(),
                &pn,
                "CacheSrv.DumpShard",
                &rpc_bytes,
                1,
            );
            // Increment the RPC idx
            rpc_idx += 1;
        }
    }
}
