#pragma once

#include <io/iovec/iovec.h>

#include <vector>

namespace sigmaos {
namespace io::transport {

class Call {
 public:
  Call(uint64_t seqno, std::shared_ptr<sigmaos::io::iovec::IOVec> in_iov,
       std::shared_ptr<sigmaos::io::iovec::IOVec> out_iov)
      : seqno(seqno), _in_iov(in_iov), _out_iov(out_iov) {}
  ~Call() {}

  uint64_t GetSeqno() const { return seqno; }
  const std::shared_ptr<sigmaos::io::iovec::IOVec> GetInIOVec() const {
    return _in_iov;
  }
  std::shared_ptr<sigmaos::io::iovec::IOVec> GetOutIOVec() const {
    return _out_iov;
  }
  // Swap IOVecs
  void SwapIOVecs() {
    auto old_out = _out_iov;
    auto old_in = _in_iov;
    _in_iov = old_out;
    _out_iov = old_in;
  }

 private:
  uint64_t seqno;
  std::shared_ptr<sigmaos::io::iovec::IOVec> _in_iov;
  std::shared_ptr<sigmaos::io::iovec::IOVec> _out_iov;
};

};  // namespace io::transport
};  // namespace sigmaos
