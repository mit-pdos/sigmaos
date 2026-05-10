use proto::s3;
use protobuf::Message;
use sigmaos;
use std::os::raw::c_char;
use std::slice;

// Boot script for imgrec: fetches model (rpcIdx=0) and image (rpcIdx=1) from
// S3 into SPProxy's delegated RPC store. Does not call recv_rpc — leaves
// results for the inference proc to retrieve via recv_delegated_rpc.
//
// Input buffer format (5 u32 LE lengths, then 5 strings):
//   [0..4]   img_bucket_len   (u32 LE)
//   [4..8]   img_key_len      (u32 LE)
//   [8..12]  model_bucket_len (u32 LE)
//   [12..16] model_key_len    (u32 LE)
//   [16..20] kid_len          (u32 LE)
//   followed by: img_bucket, img_key, model_bucket, model_key, kid
#[export_name = "boot"]
pub fn boot(b: *mut c_char, buf_sz: usize) {
    let buf: &mut [u8] = unsafe { slice::from_raw_parts_mut(b as *mut u8, buf_sz) };

    let img_bucket_len = u32::from_le_bytes(buf[0..4].try_into().unwrap()) as usize;
    let img_key_len = u32::from_le_bytes(buf[4..8].try_into().unwrap()) as usize;
    let model_bucket_len = u32::from_le_bytes(buf[8..12].try_into().unwrap()) as usize;
    let model_key_len = u32::from_le_bytes(buf[12..16].try_into().unwrap()) as usize;
    let kid_len = u32::from_le_bytes(buf[16..20].try_into().unwrap()) as usize;

    // Copy strings to owned before send_rpc overwrites the buffer.
    let mut off = 20;
    let img_bucket = str::from_utf8(&buf[off..off + img_bucket_len]).unwrap().to_string();
    off += img_bucket_len;
    let img_key = str::from_utf8(&buf[off..off + img_key_len]).unwrap().to_string();
    off += img_key_len;
    let model_bucket = str::from_utf8(&buf[off..off + model_bucket_len]).unwrap().to_string();
    off += model_bucket_len;
    let model_key = str::from_utf8(&buf[off..off + model_key_len]).unwrap().to_string();
    off += model_key_len;
    let kid = str::from_utf8(&buf[off..off + kid_len]).unwrap().to_string();
    let pn = "name/s3/".to_owned() + &kid;

    // Fetch model at rpcIdx=0 (skip if bucket/key are empty — model comes from local FS).
    if !model_bucket.is_empty() && !model_key.is_empty() {
        let mut req = s3::GetReq::new();
        req.bucket = model_bucket;
        req.key = model_key;
        sigmaos::send_rpc(buf, 0, &pn, "S3RpcAPI.GetObject", &req.write_to_bytes().unwrap(), 2);
    }

    // Fetch image at rpcIdx=1.
    let mut req = s3::GetReq::new();
    req.bucket = img_bucket;
    req.key = img_key;
    sigmaos::send_rpc(buf, 1, &pn, "S3RpcAPI.GetObject", &req.write_to_bytes().unwrap(), 2);

    sigmaos::exit(buf, sigmaos::EXIT_STATUS_OK, sigmaos::EXIT_MSG_OK);
}
