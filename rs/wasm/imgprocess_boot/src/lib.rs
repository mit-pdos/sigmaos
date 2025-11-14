use proto::s3;
use protobuf::Message;
use sigmaos;
use std::os::raw::c_char;
use std::slice;

#[export_name = "boot"]
pub fn boot(b: *mut c_char, buf_sz: usize) {
    // Convert the shared buffer to a slice
    let buf: &mut [u8] = unsafe { slice::from_raw_parts_mut(b as *mut u8, buf_sz) };
    // Get the input arguments to the boot script
    let bucket_len: usize = u32::from_le_bytes(buf[0..4].try_into().unwrap())
        .try_into()
        .unwrap();
    let key_len: usize = u32::from_le_bytes(buf[4..8].try_into().unwrap())
        .try_into()
        .unwrap();
    let kid_len: usize = u32::from_le_bytes(buf[8..12].try_into().unwrap())
        .try_into()
        .unwrap();
    let mut off: usize = 12;
    let bucket = str::from_utf8(&buf[off..off + bucket_len]).unwrap();
    off += bucket_len;
    let key = str::from_utf8(&buf[off..off + key_len]).unwrap();
    off += key_len;
    let kid = str::from_utf8(&buf[off..off + kid_len]).unwrap();
    // Create a multi_get request for each server
    let mut get_req = s3::GetReq::new();
    get_req.bucket = bucket.to_string();
    get_req.key = key.to_string();
    let rpc_bytes = get_req.write_to_bytes().unwrap();
    let pn = "name/s3/".to_owned() + &kid;
    sigmaos::send_rpc(buf, 0, &pn, "S3RpcAPI.GetObject", &rpc_bytes, 2);
}
