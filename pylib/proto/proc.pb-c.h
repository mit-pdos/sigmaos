/* Generated by the protocol buffer compiler.  DO NOT EDIT! */
/* Generated from: pylib/proto/proc.proto */

#ifndef PROTOBUF_C_pylib_2fproto_2fproc_2eproto__INCLUDED
#define PROTOBUF_C_pylib_2fproto_2fproc_2eproto__INCLUDED

#include <protobuf-c/protobuf-c.h>

PROTOBUF_C__BEGIN_DECLS

#if PROTOBUF_C_VERSION_NUMBER < 1003000
# error This file was generated by a newer version of protoc-c which is incompatible with your libprotobuf-c headers. Please update your headers.
#elif 1003003 < PROTOBUF_C_MIN_COMPILER_VERSION
# error This file was generated by an older version of protoc-c which is incompatible with your libprotobuf-c headers. Please regenerate this file with a newer version of protoc-c.
#endif

#include "pylib/proto/timestamp.pb-c.h"
#include "pylib/proto/sigmap.pb-c.h"

typedef struct _ProcSeqno ProcSeqno;
typedef struct _ProcEnvProto ProcEnvProto;
typedef struct _ProcEnvProto__EtcdEndpointsEntry ProcEnvProto__EtcdEndpointsEntry;
typedef struct _ProcEnvProto__SecretsMapEntry ProcEnvProto__SecretsMapEntry;
typedef struct _ProcProto ProcProto;
typedef struct _ProcProto__EnvEntry ProcProto__EnvEntry;


/* --- enums --- */


/* --- messages --- */

struct  _ProcSeqno
{
  ProtobufCMessage base;
  uint64_t epoch;
  uint64_t seqno;
  char *procqid;
  char *mschedid;
};
#define PROC_SEQNO__INIT \
 { PROTOBUF_C_MESSAGE_INIT (&proc_seqno__descriptor) \
    , 0, 0, (char *)protobuf_c_empty_string, (char *)protobuf_c_empty_string }


struct  _ProcEnvProto__EtcdEndpointsEntry
{
  ProtobufCMessage base;
  char *key;
  TendpointProto *value;
};
#define PROC_ENV_PROTO__ETCD_ENDPOINTS_ENTRY__INIT \
 { PROTOBUF_C_MESSAGE_INIT (&proc_env_proto__etcd_endpoints_entry__descriptor) \
    , (char *)protobuf_c_empty_string, NULL }


struct  _ProcEnvProto__SecretsMapEntry
{
  ProtobufCMessage base;
  char *key;
  SecretProto *value;
};
#define PROC_ENV_PROTO__SECRETS_MAP_ENTRY__INIT \
 { PROTOBUF_C_MESSAGE_INIT (&proc_env_proto__secrets_map_entry__descriptor) \
    , (char *)protobuf_c_empty_string, NULL }


struct  _ProcEnvProto
{
  ProtobufCMessage base;
  char *pidstr;
  char *program;
  char *realmstr;
  Tprincipal *principal;
  char *procdir;
  char *parentdir;
  size_t n_etcdendpoints;
  ProcEnvProto__EtcdEndpointsEntry **etcdendpoints;
  char *outercontaineripstr;
  char *innercontaineripstr;
  char *kernelid;
  char *buildtag;
  char *perf;
  char *debug;
  char *procdpidstr;
  protobuf_c_boolean privileged;
  int32_t howint;
  Google__Protobuf__Timestamp *spawntimepb;
  char *strace;
  TendpointProto *mschedendpointproto;
  TendpointProto *namedendpointproto;
  protobuf_c_boolean usespproxy;
  protobuf_c_boolean usedialproxy;
  size_t n_secretsmap;
  ProcEnvProto__SecretsMapEntry **secretsmap;
  size_t n_sigmapath;
  char **sigmapath;
  size_t n_kernels;
  char **kernels;
  char *realmswitchstr;
  char *version;
  char *fail;
};
#define PROC_ENV_PROTO__INIT \
 { PROTOBUF_C_MESSAGE_INIT (&proc_env_proto__descriptor) \
    , (char *)protobuf_c_empty_string, (char *)protobuf_c_empty_string, (char *)protobuf_c_empty_string, NULL, (char *)protobuf_c_empty_string, (char *)protobuf_c_empty_string, 0,NULL, (char *)protobuf_c_empty_string, (char *)protobuf_c_empty_string, (char *)protobuf_c_empty_string, (char *)protobuf_c_empty_string, (char *)protobuf_c_empty_string, (char *)protobuf_c_empty_string, (char *)protobuf_c_empty_string, 0, 0, NULL, (char *)protobuf_c_empty_string, NULL, NULL, 0, 0, 0,NULL, 0,NULL, 0,NULL, (char *)protobuf_c_empty_string, (char *)protobuf_c_empty_string, (char *)protobuf_c_empty_string }


struct  _ProcProto__EnvEntry
{
  ProtobufCMessage base;
  char *key;
  char *value;
};
#define PROC_PROTO__ENV_ENTRY__INIT \
 { PROTOBUF_C_MESSAGE_INIT (&proc_proto__env_entry__descriptor) \
    , (char *)protobuf_c_empty_string, (char *)protobuf_c_empty_string }


struct  _ProcProto
{
  ProtobufCMessage base;
  ProcEnvProto *procenvproto;
  size_t n_args;
  char **args;
  size_t n_env;
  ProcProto__EnvEntry **env;
  uint32_t typeint;
  uint32_t mcpuint;
  uint32_t memint;
};
#define PROC_PROTO__INIT \
 { PROTOBUF_C_MESSAGE_INIT (&proc_proto__descriptor) \
    , NULL, 0,NULL, 0,NULL, 0, 0, 0 }


/* ProcSeqno methods */
void   proc_seqno__init
                     (ProcSeqno         *message);
size_t proc_seqno__get_packed_size
                     (const ProcSeqno   *message);
size_t proc_seqno__pack
                     (const ProcSeqno   *message,
                      uint8_t             *out);
size_t proc_seqno__pack_to_buffer
                     (const ProcSeqno   *message,
                      ProtobufCBuffer     *buffer);
ProcSeqno *
       proc_seqno__unpack
                     (ProtobufCAllocator  *allocator,
                      size_t               len,
                      const uint8_t       *data);
void   proc_seqno__free_unpacked
                     (ProcSeqno *message,
                      ProtobufCAllocator *allocator);
/* ProcEnvProto__EtcdEndpointsEntry methods */
void   proc_env_proto__etcd_endpoints_entry__init
                     (ProcEnvProto__EtcdEndpointsEntry         *message);
/* ProcEnvProto__SecretsMapEntry methods */
void   proc_env_proto__secrets_map_entry__init
                     (ProcEnvProto__SecretsMapEntry         *message);
/* ProcEnvProto methods */
void   proc_env_proto__init
                     (ProcEnvProto         *message);
size_t proc_env_proto__get_packed_size
                     (const ProcEnvProto   *message);
size_t proc_env_proto__pack
                     (const ProcEnvProto   *message,
                      uint8_t             *out);
size_t proc_env_proto__pack_to_buffer
                     (const ProcEnvProto   *message,
                      ProtobufCBuffer     *buffer);
ProcEnvProto *
       proc_env_proto__unpack
                     (ProtobufCAllocator  *allocator,
                      size_t               len,
                      const uint8_t       *data);
void   proc_env_proto__free_unpacked
                     (ProcEnvProto *message,
                      ProtobufCAllocator *allocator);
/* ProcProto__EnvEntry methods */
void   proc_proto__env_entry__init
                     (ProcProto__EnvEntry         *message);
/* ProcProto methods */
void   proc_proto__init
                     (ProcProto         *message);
size_t proc_proto__get_packed_size
                     (const ProcProto   *message);
size_t proc_proto__pack
                     (const ProcProto   *message,
                      uint8_t             *out);
size_t proc_proto__pack_to_buffer
                     (const ProcProto   *message,
                      ProtobufCBuffer     *buffer);
ProcProto *
       proc_proto__unpack
                     (ProtobufCAllocator  *allocator,
                      size_t               len,
                      const uint8_t       *data);
void   proc_proto__free_unpacked
                     (ProcProto *message,
                      ProtobufCAllocator *allocator);
/* --- per-message closures --- */

typedef void (*ProcSeqno_Closure)
                 (const ProcSeqno *message,
                  void *closure_data);
typedef void (*ProcEnvProto__EtcdEndpointsEntry_Closure)
                 (const ProcEnvProto__EtcdEndpointsEntry *message,
                  void *closure_data);
typedef void (*ProcEnvProto__SecretsMapEntry_Closure)
                 (const ProcEnvProto__SecretsMapEntry *message,
                  void *closure_data);
typedef void (*ProcEnvProto_Closure)
                 (const ProcEnvProto *message,
                  void *closure_data);
typedef void (*ProcProto__EnvEntry_Closure)
                 (const ProcProto__EnvEntry *message,
                  void *closure_data);
typedef void (*ProcProto_Closure)
                 (const ProcProto *message,
                  void *closure_data);

/* --- services --- */


/* --- descriptors --- */

extern const ProtobufCMessageDescriptor proc_seqno__descriptor;
extern const ProtobufCMessageDescriptor proc_env_proto__descriptor;
extern const ProtobufCMessageDescriptor proc_env_proto__etcd_endpoints_entry__descriptor;
extern const ProtobufCMessageDescriptor proc_env_proto__secrets_map_entry__descriptor;
extern const ProtobufCMessageDescriptor proc_proto__descriptor;
extern const ProtobufCMessageDescriptor proc_proto__env_entry__descriptor;

PROTOBUF_C__END_DECLS


#endif  /* PROTOBUF_C_pylib_2fproto_2fproc_2eproto__INCLUDED */
