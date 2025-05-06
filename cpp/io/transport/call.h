#pragma once

#include <vector>

namespace sigmaos {
namespace io::transport {

class Call {
  public:
  Call(uint64_t seqno, std::vector<std::string *> &in_iov, std::vector<std::string *> &out_iov) : 
    seqno(seqno), _in_iov(in_iov), _out_iov(out_iov) {}
  ~Call() {}

  uint64_t GetSeqno() const { return seqno; }
  const std::vector<std::string *> &GetInIOVec() const { return _in_iov; }
  std::vector<std::string *> &GetOutIOVec() const { return _out_iov; }

  private:
  uint64_t seqno;
  // TODO: does this need to be a ptr?
  std::vector<std::string *> &_in_iov;
  std::vector<std::string *> &_out_iov;
};

};
};
