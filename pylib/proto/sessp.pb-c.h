/* Generated by the protocol buffer compiler.  DO NOT EDIT! */
/* Generated from: pylib/proto/sessp.proto */

#ifndef PROTOBUF_C_pylib_2fproto_2fsessp_2eproto__INCLUDED
#define PROTOBUF_C_pylib_2fproto_2fsessp_2eproto__INCLUDED

#include <protobuf-c/protobuf-c.h>

PROTOBUF_C__BEGIN_DECLS

#if PROTOBUF_C_VERSION_NUMBER < 1003000
# error This file was generated by a newer version of protoc-c which is incompatible with your libprotobuf-c headers. Please update your headers.
#elif 1003003 < PROTOBUF_C_MIN_COMPILER_VERSION
# error This file was generated by an older version of protoc-c which is incompatible with your libprotobuf-c headers. Please regenerate this file with a newer version of protoc-c.
#endif


typedef struct _Fcall Fcall;


/* --- enums --- */


/* --- messages --- */

struct  _Fcall
{
  ProtobufCMessage base;
  uint32_t type;
  uint64_t session;
  uint64_t seqno;
  uint32_t len;
  uint32_t nvec;
};
#define FCALL__INIT \
 { PROTOBUF_C_MESSAGE_INIT (&fcall__descriptor) \
    , 0, 0, 0, 0, 0 }


/* Fcall methods */
void   fcall__init
                     (Fcall         *message);
size_t fcall__get_packed_size
                     (const Fcall   *message);
size_t fcall__pack
                     (const Fcall   *message,
                      uint8_t             *out);
size_t fcall__pack_to_buffer
                     (const Fcall   *message,
                      ProtobufCBuffer     *buffer);
Fcall *
       fcall__unpack
                     (ProtobufCAllocator  *allocator,
                      size_t               len,
                      const uint8_t       *data);
void   fcall__free_unpacked
                     (Fcall *message,
                      ProtobufCAllocator *allocator);
/* --- per-message closures --- */

typedef void (*Fcall_Closure)
                 (const Fcall *message,
                  void *closure_data);

/* --- services --- */


/* --- descriptors --- */

extern const ProtobufCMessageDescriptor fcall__descriptor;

PROTOBUF_C__END_DECLS


#endif  /* PROTOBUF_C_pylib_2fproto_2fsessp_2eproto__INCLUDED */
