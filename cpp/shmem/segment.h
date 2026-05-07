#pragma once

#include <serr/serr.h>
#include <shmem/shmem.h>
#include <sigmap/const.h>
#include <sys/types.h>
#include <util/log/log.h>

#include <cstddef>
#include <expected>
#include <format>
#include <string>

namespace sigmaos {
namespace shmem {

const std::string SHMEM = "SHMEM";
const std::string SHMEM_ERR = SHMEM + sigmaos::util::log::ERR;

class Segment {
 public:
  Segment(const std::string &id, size_t size, bool is_blink = false)
      : _id_str(id), _fd(-1), _size(size), _buf(nullptr), _is_blink(is_blink) {}
  ~Segment() {
    auto res = Destroy();
    if (!res.has_value()) {
      fatal("Err when Destroying shmem: {}", res.error().String());
    }
  }

  std::expected<int, sigmaos::serr::Error> Init();
  std::expected<int, sigmaos::serr::Error> Destroy();
  void *GetBuf() { return _buf; }

 private:
  std::string _id_str;
  int _fd;  // File descriptor for POSIX shared memory
  size_t _size;
  void *_buf;
  bool _is_blink;
  // Used for logger initialization
  static bool _l;
  static bool _l_e;
};

};  // namespace shmem
};  // namespace sigmaos
