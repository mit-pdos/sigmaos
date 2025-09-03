#pragma once

#include <google/protobuf/message.h>
#include <io/demux/clnt.h>
#include <io/iovec/iovec.h>
#include <rpc/proto/rpc.pb.h>
#include <serr/serr.h>
#include <util/log/log.h>

#include <atomic>
#include <expected>

namespace sigmaos {
namespace rpc {

// A channel/connection over which to make RPCs
class Channel {
 public:
  virtual std::expected<std::shared_ptr<sigmaos::io::transport::Call>,
                        sigmaos::serr::Error>
  SendReceive(std::shared_ptr<sigmaos::io::transport::Call> call) = 0;
  virtual std::expected<int, sigmaos::serr::Error> Init() = 0;
  virtual std::expected<int, sigmaos::serr::Error> Close() = 0;
  virtual bool IsInitialized() = 0;
  virtual bool IsClosed() = 0;
};

};  // namespace rpc
};  // namespace sigmaos
