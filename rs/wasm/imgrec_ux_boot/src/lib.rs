use sigmaos;
use std::os::raw::c_char;
use std::slice;
use std::str;

// Boot script for imgrec with UX: fetches model (rpcIdx=0) and image (rpcIdx=1)
// from the UX filesystem into SPProxy's delegated RPC store. Does not call
// recv_rpc — leaves results for the inference proc to retrieve via
// recv_delegated_rpc.
//
// Input buffer format (3 u32 LE lengths, then 3 strings):
//   [0..4]   img_path_len   (u32 LE)
//   [4..8]   model_path_len (u32 LE)
//   [8..12]  kid_len        (u32 LE)
//   followed by: img_path, model_path, kid
//
// UXReq protobuf encoding (field 1 = path, wire type 2):
//   hand-rolled to avoid pulling in the full proto crate.

fn encode_varint(mut v: u64, out: &mut Vec<u8>) {
    loop {
        let byte = (v & 0x7F) as u8;
        v >>= 7;
        if v == 0 {
            out.push(byte);
            break;
        } else {
            out.push(byte | 0x80);
        }
    }
}

// Encode UXReq { path: s } as protobuf bytes (field 1, wire type 2).
fn encode_ux_req(s: &str) -> Vec<u8> {
    let mut out = Vec::new();
    out.push(0x0A); // tag: field 1, wire type 2
    encode_varint(s.len() as u64, &mut out);
    out.extend_from_slice(s.as_bytes());
    out
}

#[export_name = "boot"]
pub fn boot(b: *mut c_char, buf_sz: usize) {
    let buf: &mut [u8] = unsafe { slice::from_raw_parts_mut(b as *mut u8, buf_sz) };

    let img_path_len = u32::from_le_bytes(buf[0..4].try_into().unwrap()) as usize;
    let model_path_len = u32::from_le_bytes(buf[4..8].try_into().unwrap()) as usize;
    let kid_len = u32::from_le_bytes(buf[8..12].try_into().unwrap()) as usize;

    let mut off = 12;
    let img_path = str::from_utf8(&buf[off..off + img_path_len]).unwrap().to_string();
    off += img_path_len;
    let model_path = str::from_utf8(&buf[off..off + model_path_len]).unwrap().to_string();
    off += model_path_len;
    let kid = str::from_utf8(&buf[off..off + kid_len]).unwrap().to_string();
    let pn = "name/ux/".to_owned() + &kid;

    // Fetch model at rpcIdx=0 (skip if path is empty — model comes from local FS).
    if !model_path.is_empty() {
        let req_bytes = encode_ux_req(&model_path);
        sigmaos::send_rpc(buf, 0, &pn, "UXRpcAPI.GetFile", &req_bytes, 2);
    }

    // Fetch image at rpcIdx=1.
    let req_bytes = encode_ux_req(&img_path);
    sigmaos::send_rpc(buf, 1, &pn, "UXRpcAPI.GetFile", &req_bytes, 2);

    sigmaos::exit(buf, sigmaos::EXIT_STATUS_OK, sigmaos::EXIT_MSG_OK);
}
