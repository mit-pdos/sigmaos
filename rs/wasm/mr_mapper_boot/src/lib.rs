// Cosandbox boot script for MR mappers: pre-fetches the mapper's input
// splits by issuing one ranged UX/S3 get per split, depositing the replies
// in the delegated-RPC store at rpcIdx = split index (bin order). The mapper
// pairs them up via getput.GetPutReader.
//
// Boot input layout (built by the MR coordinator):
//   [0..4]  n            (u32 LE): number of splits
//   [4.. ]  EncodeArgs([kid, typ_0, a_0, b_0, off_0, cnt_0, ..., typ_{n-1}, ...])
// where EncodeArgs writes all (1+5n) u32-LE string lengths first, then all
// string bodies. Per split: typ is "s3" (a=bucket, b=key) or "ux" (a=path
// relative to the UX root, b unused); off and cnt are decimal strings for
// the split's read window (see mr.SplitReadWindow).

use sigmaos;
use std::os::raw::c_char;
use std::slice;

#[export_name = "boot"]
pub fn boot(b: *mut c_char, buf_sz: usize) {
    let buf: &mut [u8] = unsafe { slice::from_raw_parts_mut(b as *mut u8, buf_sz) };
    let n: usize = u32::from_le_bytes(buf[0..4].try_into().unwrap())
        .try_into()
        .unwrap();
    let nstr = 1 + 5 * n;
    let base = 4;
    // String lengths, then string body offsets
    let mut lens = Vec::with_capacity(nstr);
    for i in 0..nstr {
        let l: usize = u32::from_le_bytes(buf[base + 4 * i..base + 4 * i + 4].try_into().unwrap())
            .try_into()
            .unwrap();
        lens.push(l);
    }
    // Extract all strings upfront: send_rpc needs the buffer mutably.
    let mut strs = Vec::with_capacity(nstr);
    let mut off = base + 4 * nstr;
    for i in 0..nstr {
        strs.push(
            std::str::from_utf8(&buf[off..off + lens[i]])
                .unwrap()
                .to_owned(),
        );
        off += lens[i];
    }

    let kid = &strs[0];
    let pn_s3 = "name/s3/".to_owned() + kid;
    let pn_ux = "name/ux/".to_owned() + kid;

    for i in 0..n {
        let typ = &strs[1 + 5 * i];
        let a = &strs[1 + 5 * i + 1];
        let b_str = &strs[1 + 5 * i + 2];
        let off: u64 = strs[1 + 5 * i + 3].parse().unwrap();
        let cnt: u64 = strs[1 + 5 * i + 4].parse().unwrap();
        if typ == "s3" {
            let req = encode_s3_req(&a, &b_str, off, cnt);
            sigmaos::send_rpc(buf, i as u64, &pn_s3, "S3RpcAPI.GetObject", &req, 2);
        } else {
            let req = encode_ux_req(&a, off, cnt);
            sigmaos::send_rpc(buf, i as u64, &pn_ux, "UXRpcAPI.GetFile", &req, 2);
        }
    }
    sigmaos::exit(buf, sigmaos::EXIT_STATUS_OK, sigmaos::EXIT_MSG_OK);
}

// Append a protobuf base-128 varint.
fn put_varint(out: &mut Vec<u8>, mut v: u64) {
    loop {
        if v < 0x80 {
            out.push(v as u8);
            break;
        }
        out.push((v as u8) | 0x80);
        v >>= 7;
    }
}

// Append a length-delimited (wire type 2) field.
fn put_bytes_field(out: &mut Vec<u8>, field: u64, b: &[u8]) {
    put_varint(out, (field << 3) | 2);
    put_varint(out, b.len() as u64);
    out.extend_from_slice(b);
}

// Append a varint (wire type 0) field; zero values are omitted (proto3
// default).
fn put_uint_field(out: &mut Vec<u8>, field: u64, v: u64) {
    if v == 0 {
        return;
    }
    put_varint(out, field << 3);
    put_varint(out, v);
}

// S3Req{bucket=1, key=2, offset=5, count=6}
fn encode_s3_req(bucket: &str, key: &str, off: u64, cnt: u64) -> Vec<u8> {
    let mut out = Vec::with_capacity(16 + bucket.len() + key.len());
    put_bytes_field(&mut out, 1, bucket.as_bytes());
    put_bytes_field(&mut out, 2, key.as_bytes());
    put_uint_field(&mut out, 5, off);
    put_uint_field(&mut out, 6, cnt);
    out
}

// UXReq{path=1, offset=3, count=4}
fn encode_ux_req(path: &str, off: u64, cnt: u64) -> Vec<u8> {
    let mut out = Vec::with_capacity(12 + path.len());
    put_bytes_field(&mut out, 1, path.as_bytes());
    put_uint_field(&mut out, 3, off);
    put_uint_field(&mut out, 4, cnt);
    out
}
