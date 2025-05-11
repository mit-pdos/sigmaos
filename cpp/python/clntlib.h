#pragma once

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

#include <iostream>
#include <memory>
#include <expected>

#include <proxy/sigmap/sigmap.h>
#include <proxy/sigmap/proto/spproxy.pb.h>

extern "C" {
  typedef struct {
    uint32_t type;
    uint32_t version;
    uint64_t path;
  } CTqidProto;

  typedef struct {
    uint32_t type;
    uint32_t dev;
    CTqidProto qid;
    uint32_t mode;
    uint32_t atime;     // last access time in seconds
    uint32_t mtime;     // last modified time in seconds
    uint64_t length;    // file length in bytes
    const char* name;   // file name
    const char* uid;    // owner name
    const char* gid;    // group name
    const char* muid;   // name of last user that modified the file
  } CTstatProto;

  void init_socket();
  // Stubs
  void close_fd_stub(int fd);
  CTstatProto* stat_stub(char* pn);
  int create_stub(char* pn, uint32_t perm, uint32_t mode);
  int open_stub(char *pn, uint32_t mode, bool wait);
  void rename_stub(char* src, char* dst);
  void remove_stub(char* pn); 
  char* get_file_stub(char* pn);
  uint32_t put_file_stub(char* pn, uint32_t perm, uint32_t mode, char* data, uint64_t o, uint64_t l); 
  uint32_t read_stub(int fd, char* b);
  uint32_t pread_stub(int fd, char* b, uint64_t o);
  uint32_t write_stub(int fd, char* b);
  void seek_stub(int fd, uint64_t o);
  // TODO: everything after Seek in cpp/proxy/sigmap/sigmap.h
  // ProcClnt API
  void started();
  void exited(uint32_t status, char* msg);
  void wait_evict();
}
