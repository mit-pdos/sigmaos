#pragma once

#include <expected>
#include <atomic>

#include <google/protobuf/message.h>

#include <util/log/log.h>
#include <serr/serr.h>
#include <io/demux/clnt.h>
#include <io/iovec/iovec.h>
#include <rpc/proto/rpc.pb.h>
#include <rpc/channel.h>
#include <rpc/rpc.h>
#include <proxy/sigmap/sigmap.h>

namespace sigmaos {
namespace rpc::spchannel {

const std::string SPCHAN = "SPCHAN";
const std::string SPCHAN_ERR = SPCHAN + sigmaos::util::log::ERR;

// A channel/connection over which to make RPCs
class Channel : public sigmaos::rpc::Channel {
  public:
// TODO: constructor from endpoint
  Channel(std::string srv_pn, std::shared_ptr<sigmaos::proxy::sigmap::Clnt> sp_clnt) : _initialized(false), _srv_pn(srv_pn), _sp_clnt(sp_clnt), _closed(false) {}
  ~Channel() {}
  // Initialize the channel
  std::expected<int, sigmaos::serr::Error> Init();
  std::expected<std::shared_ptr<sigmaos::io::transport::Call>, sigmaos::serr::Error> SendReceive(std::shared_ptr<sigmaos::io::transport::Call> call);
  std::expected<int, sigmaos::serr::Error> Close();
  bool IsClosed();
  bool IsInitialized();
  private:
  bool _initialized;
  std::string _srv_pn;
  std::string _sid;
  int _fd;
  std::shared_ptr<sigmaos::proxy::sigmap::Clnt> _sp_clnt;
  bool _closed;
  // Used for logger initialization
  static bool _l;
  static bool _l_e;
};

};
};
