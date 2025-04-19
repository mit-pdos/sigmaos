#include <sys/stat.h>
#include <sys/socket.h>
#include <sys/un.h>
#include <stdio.h>
#include <stdlib.h>
#include <stdarg.h>
#include <dlfcn.h>
#include <dirent.h>
#include <string.h>
#include <unistd.h>
#include <errno.h>
#include <stdint.h>
#include <protobuf-c/protobuf-c.h>
#include "pylib/proto/rpc.pb-c.h"
#include "pylib/proto/spproxy.pb-c.h"

const char* api_socket_env_var = "SIGMA_PYAPI_FD";
int api_sfd = 0; 

const char* spproxy_env_var = "SIGMA_SPPROXY_FD";
int spproxy_sfd = 0;

uint64_t seqno;

/************
 * Proc API *
 ************/

void init_socket() 
{
  const char* sfd_str = getenv(api_socket_env_var);
  if (sfd_str == NULL) {
    exit(-1);
  }
  api_sfd = atoi(sfd_str);

  const char* spproxy_sfd_str = getenv(spproxy_env_var);
  if (spproxy_sfd_str == NULL) {
    exit(-1);
  }
  spproxy_sfd = atoi(spproxy_sfd_str);

  seqno = 593;
}

void started()
{
  char response[2];
  write(api_sfd, "api/started\n", 12);
  read(api_sfd, response, 1);
  while (response[0] != 'd') {
    read(api_sfd, response, 1);
  }
}

void exited()
{
  char response[2];
  write(api_sfd, "api/exited\n", 11);
  read(api_sfd, response, 1);
  while (response[0] != 'd') {
    read(api_sfd, response, 1);
  }
}

/*******************
 * SPProxy Helpers *
 *******************/

/*
 * util/io/frame/frame.go: writeRawBuffer(wr io.Writer, buf sessp.Tframe) *serr.Err
 */
void write_raw_buffer(char* buf, uint32_t buf_len) {
  write(spproxy_sfd, buf, buf_len);
}

/*
 * util/io/frame/frame.go: WriteSeqno(seqno sessp.Tseqno, wr io.Writer) *serr.Err
 */
void write_seqno(uint64_t seqno) {
  // Convert to little Endian
  uint8_t bytes[8];
  for (int i = 0; i < 8; i++) {
    bytes[i] = (seqno >> (i * 8)) & 0xFF;
  }

  write(spproxy_sfd, bytes, 8);
}

/*
 * util/io/frame/frame.go: WriteFrame(wr io.Writer, frame sessp.Tframe) *serr.Err
 */
void write_frame(char* frame, uint32_t frame_len) {
  // Write frame_len + 4
  uint8_t bytes[4];
  for (int i = 0; i < 4; i++) {
    bytes[i] = ((frame_len + 4) >> (i * 8)) & 0xFF;
  }
  write(spproxy_sfd, bytes, 4);

  write_raw_buffer(frame, frame_len);
}

/*
 * util/io/frame/frame.go: WriteFrames(wr io.Writer, iov sessp.IoVec) *serr.Err
 */
void write_frames(char** frames, uint32_t num_frames, uint32_t* frame_lens) {
  // Write num_frames
  uint8_t bytes[4];
  for (int i = 0; i < 4; i++) {
    bytes[i] = ((num_frames + 4) >> (i * 8)) & 0xFF;
  }
  write(spproxy_sfd, bytes, 4);

  for (uint64_t i = 0; i < num_frames; i++) {
    write_frame(frames[i], frame_lens[i]);
  }
}

/*
 * proxy/sigmap/transport/transport.go: WriteCall(c demux.CallI) *serr.Err
 */
void write_call(ProtobufCBinaryData* c, uint32_t num_frames) {
  write_seqno(seqno);

  // Convert ProtobufCBinaryData* to char**
  char** frames = malloc(num_frames * sizeof(char *));
  uint32_t* frame_lens = malloc(num_frames * sizeof(uint32_t));
  for (uint32_t i = 0; i < num_frames; i++) {
    frame_lens[i] = c[i].len;
    frames[i] = (char *) c[i].data;
  }

  write_frames(frames, num_frames, frame_lens);
}

/*
 * rpc/blob.go: GetBlob(msg proto.Message) *rpcproto.Blob
 */
Blob* get_blob(ProtobufCMessage* msg) {
  Blob* blob = NULL;
  if (strcmp(msg->descriptor->name, "Blob") == 0) {
    return (Blob *) msg;
  } else if (strcmp(msg->descriptor->name, "SigmaDataRep") == 0) {
    return ((SigmaDataRep *) msg)->blob;
  } else if (strcmp(msg->descriptor->name, "SigmaPutFileReq") == 0) {
    return ((SigmaPutFileReq *) msg)->blob;
  } else if (strcmp(msg->descriptor->name, "SigmaWriteReq") == 0) {
    return ((SigmaWriteReq *) msg)->blob;
  }
  return blob;
}

/*
 * rpc/clnt/clnt.go: RPC(method string, arg proto.Message, res proto.Message) error
 */
void rpc(char* method, ProtobufCMessage* arg, ProtobufCMessage* res) {
  Blob* inblob = get_blob(arg);
  ProtobufCBinaryData* iniov = NULL;
  size_t n_iniov = 0;
  if (inblob != NULL) {
    iniov = inblob->iov;
    n_iniov = inblob->n_iov;
    inblob->iov = NULL;
    inblob->n_iov = 0;
  }

  // Marshal the protobuf message
  size_t arg_len = protobuf_c_message_get_packed_size(arg);
  char* arg_buf = malloc(arg_len);
  protobuf_c_message_pack(arg, (uint8_t *) arg_buf);

  // Prepend 2 empty slots for the out iovec
  size_t n_outiov = 2;
  // Get the reply's blob if it has one
  Blob* outblob = get_blob(res);
  if (outblob != NULL) {
    n_outiov += outblob->n_iov;
  }
  ProtobufCBinaryData* outiov = calloc(n_outiov, sizeof(ProtobufCBinaryData));
  if (outblob != NULL) {
    for (size_t i = 0; i < outblob->n_iov; i++) {
      outiov[i + 2] = outblob->iov[i];
    }
  }
  
  // rpc(method string, iniov sessp.IoVec, outiov sessp.IoVec) (*rpcproto.Rep, error) 
  Req req = REQ__INIT;
  req.method = method;
  size_t req_len = protobuf_c_message_get_packed_size((ProtobufCMessage *) &req);
  char* req_buf = malloc(req_len);
  protobuf_c_message_pack((ProtobufCMessage *) &req, (uint8_t *) req_buf);

  // Prepare arguments for SendReceive
  ProtobufCBinaryData* rpc_iniov = malloc((n_iniov + 2) * sizeof(ProtobufCBinaryData));
  ProtobufCBinaryData b;
  b.data = (uint8_t *) req_buf;
  b.len = req_len;
  ProtobufCBinaryData a;
  a.data = (uint8_t *) arg_buf;
  a.len = arg_len;
  rpc_iniov[0] = b;
  rpc_iniov[1] = a;
  for (size_t i = 0; i < n_iniov; i++) {
    rpc_iniov[i + 2] = iniov[i];
  }

  write_call(rpc_iniov, n_iniov + 2);
}

/***************
 * SigmaOS API *
 ***************/

// proxy/sigmap/clnt/stubs.go: Stat(path string) (*sp.Tstat, error)
void stat_stub(char* path) {
  SigmaPathReq req = SIGMA_PATH_REQ__INIT; 
  req.path = path;
  SigmaStatRep rep = SIGMA_STAT_REP__INIT;
  rpc("SPProxySrvAPI.Stat", (ProtobufCMessage *) &req, (ProtobufCMessage *) &rep);
}
