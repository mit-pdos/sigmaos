#include <fcntl.h>
#include <shmem/segment.h>
#include <sys/mman.h>
#include <sys/stat.h>
#include <unistd.h>

#include <cstring>

namespace sigmaos {
namespace shmem {

bool Segment::_l = sigmaos::util::log::init_logger(SHMEM);
bool Segment::_l_e = sigmaos::util::log::init_logger(SHMEM_ERR);

std::expected<int, sigmaos::serr::Error> Segment::Init() {
  if (_is_blink) {
    std::string path = "/shm/" + _id_str;
    _fd = open(path.c_str(), O_RDWR, 0666);
    if (_fd == -1) {
      return std::unexpected(
          sigmaos::serr::Error(sigmaos::serr::Terror::TErrError,
                               std::format("err open: {}", path)));
    }
  } else {
    std::string name = "/" + _id_str;
    _fd = shm_open(name.c_str(), O_RDWR, 0666);
    if (_fd == -1) {
      return std::unexpected(
          sigmaos::serr::Error(sigmaos::serr::Terror::TErrError,
                               std::format("err shm_open: {}", name)));
    }
  }
  // Map the shared memory object into the process address space
  _buf = mmap(nullptr, _size, PROT_READ | PROT_WRITE, MAP_SHARED, _fd, 0);
  if (_buf == MAP_FAILED) {
    close(_fd);
    return std::unexpected(sigmaos::serr::Error(
        sigmaos::serr::Terror::TErrError, std::format("err mmap")));
  }
  return 0;
}

std::expected<int, sigmaos::serr::Error> Segment::Destroy() {
  // Unmap the shared memory
  int res = munmap(_buf, _size);
  if (res != 0) {
    return std::unexpected(sigmaos::serr::Error(
        sigmaos::serr::Terror::TErrError, std::format("err munmap")));
  }
  // Close the file descriptor
  res = close(_fd);
  if (res != 0) {
    return std::unexpected(sigmaos::serr::Error(
        sigmaos::serr::Terror::TErrError, std::format("err close")));
  }
  if (_is_blink) {
    std::string path = "/shm/" + _id_str;
    res = unlink(path.c_str());
    if (res != 0) {
      return std::unexpected(sigmaos::serr::Error(
          sigmaos::serr::Terror::TErrError, std::format("err unlink")));
    }
  } else {
    std::string name = "/" + _id_str;
    res = shm_unlink(name.c_str());
    if (res != 0) {
      return std::unexpected(sigmaos::serr::Error(
          sigmaos::serr::Terror::TErrError, std::format("err shm_unlink")));
    }
  }
  return 0;
}

};  // namespace shmem
};  // namespace sigmaos
