#pragma once

#include <vector>

#include <io/iovec/iovec.h>

namespace sigmaos {
namespace io::transport {

class Call {
  public:
  Call(uint64_t seqno, std::shared_ptr<sigmaos::io::iovec::IOVec> in_iov, std::shared_ptr<sigmaos::io::iovec::IOVec> out_iov) : 
    seqno(seqno), _in_iov(in_iov), _out_iov(out_iov) {}
  ~Call() {}

  uint64_t GetSeqno() const { return seqno; }
  const std::shared_ptr<sigmaos::io::iovec::IOVec> GetInIOVec() const { return _in_iov; }
  std::shared_ptr<sigmaos::io::iovec::IOVec> GetOutIOVec() const { return _out_iov; }

  private:
  uint64_t seqno;
  std::shared_ptr<sigmaos::io::iovec::IOVec> _in_iov;
  std::shared_ptr<sigmaos::io::iovec::IOVec> _out_iov;
};

};
};
