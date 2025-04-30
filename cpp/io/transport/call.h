#pragma once

#include <vector>

namespace sigmaos {
namespace io::transport {

class Call {
  public:
  Call(uint64_t seqno, std::vector<std::vector<unsigned char>> &in_iov, std::vector<std::vector<unsigned char>> &out_iov) : 
    seqno(seqno), _in_iov(in_iov), _out_iov(out_iov) {}
  ~Call() {}

  uint64_t GetSeqno() const { return seqno; }
  const std::vector<std::vector<unsigned char>> &GetInIOVec() const { return _in_iov; }
  std::vector<std::vector<unsigned char>> &GetOutIOVec() const { return _out_iov; }

  private:
  uint64_t seqno;
  // TODO: does this need to be a ptr?
  std::vector<std::vector<unsigned char>> &_in_iov;
  std::vector<std::vector<unsigned char>> &_out_iov;
};

};
};
