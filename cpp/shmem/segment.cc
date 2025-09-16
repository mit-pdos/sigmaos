#include <fcntl.h>
#include <shmem/segment.h>
#include <sys/stat.h>
#include <unistd.h>
#include <sys/shm.h>

#include <cstring>

namespace sigmaos {
namespace shmem {

bool Segment::_l = sigmaos::util::log::init_logger(SHMEM);
bool Segment::_l_e = sigmaos::util::log::init_logger(SHMEM_ERR);

std::expected<int, sigmaos::serr::Error> Segment::Init() {
  _id = shmget(_key, _size, 0);
  if (_id == -1) {
    return std::unexpected(
        sigmaos::serr::Error(sigmaos::serr::Terror::TErrError,
                             std::format("err shmget key {}", (uint64_t) _key)));
  }
  _buf = shmat(_id, nullptr, 0666);
  if (_buf == (void *)-1) {
    return std::unexpected(
        sigmaos::serr::Error(sigmaos::serr::Terror::TErrError,
                             std::format("err shmat")));
  }
  return 0;
}

std::expected<int, sigmaos::serr::Error> Segment::Destroy() {
  int res = shmdt(_buf);
  if (res != 0) {
    return std::unexpected(
        sigmaos::serr::Error(sigmaos::serr::Terror::TErrError,
                             std::format("err shmdt")));
  }
  res = shmctl(_id, IPC_RMID, nullptr);
  if (res != 0) {
    return std::unexpected(
        sigmaos::serr::Error(sigmaos::serr::Terror::TErrError,
                             std::format("err rmshmem")));
  }
  return 0;
}

};  // namespace shmem
};  // namespace sigmaos
