#pragma once

#include <sys/un.h>
#include <unistd.h>

#include <iostream>
#include <memory>
#include <expected>

#include <io/conn/conn.h>

namespace sigmaos {
namespace io::transport {

class Call {
  public:
  Call(uint64_t seqno, std::vector<std::vector<unsigned char>> iov) : 
    seqno(seqno), iov(iov) {}
  ~Call() {}

  uint64_t GetSeqno() const { return seqno; }
  const std::vector<std::vector<unsigned char>> &GetIOVec() const { return iov; }

  private:
  uint64_t seqno;
  // TODO: does this need to be a ptr?
  std::vector<std::vector<unsigned char>> &iov;
};

class Transport {
  public:
  Transport(std::shared_ptr<sigmaos::io::conn::UnixConn> conn) : conn(conn) {
    std::cout << "New demux clnt" << std::endl;
  }

  ~Transport() {
    std::cout << "Close transport" << std::endl;
    throw std::runtime_error("unimplemented");
  }

  std::expected<int, std::string> WriteCall(const Call &c);
  std::expected<std::shared_ptr<Call>, std::string> ReadCall(std::vector<std::vector<unsigned char>> &iov);
  std::expected<int, std::string> Close() { return conn->Close(); }

  private:
  std::shared_ptr<sigmaos::io::conn::UnixConn> conn;
};

};
};
