#include <rpc/spchannel/spchannel.h>

namespace sigmaos {
namespace rpc::spchannel {

bool Channel::_l = sigmaos::util::log::init_logger(SPCHAN);
bool Channel::_l_e = sigmaos::util::log::init_logger(SPCHAN_ERR);

// Initialize the channel & connect to the server
std::expected<int, sigmaos::serr::Error> Channel::Init() {
  // Sanity check
  if (_initialized) {
    fatal("Double-init channel to {}", _srv_pn);
  }
  std::string clone_pn = _srv_pn + "/" + sigmaos::rpc::CLONE;
  {
    auto res = _sp_clnt->GetFile(clone_pn);
    if (!res.has_value()) {
      log(SPCHAN_ERR, "Error GetFile clone file({}): {}", clone_pn, res.error().String());
      return std::unexpected(res.error());
    }
    _sid = *res.value();
  }
  {
    std::string sess_dev_pn = _srv_pn + "/" + sigmaos::rpc::RPC + "/" + _sid + "/data";
    auto res = _sp_clnt->Open(sess_dev_pn, sigmaos::sigmap::constants::ORDWR, false);
    if (!res.has_value()) {
      log(SPCHAN_ERR, "Error Open data file({}): {}", sess_dev_pn, res.error().String());
      return std::unexpected(res.error());
    }
    _fd = res.value();
  }
  _initialized = true;
  log(SPCHAN, "Successfully initialized sigmap-based RPC channel to {}", _srv_pn);
  return 0;
}

std::expected<std::shared_ptr<sigmaos::io::transport::Call>, sigmaos::serr::Error> Channel::SendReceive(std::shared_ptr<sigmaos::io::transport::Call> call) {
  // Sanity check
  if (!_initialized) {
    fatal("Use uninitialized channel");
  }
  {
    auto res = _sp_clnt->WriteRead(_fd, call->GetInIOVec(), call->GetOutIOVec());
    if (!res.has_value()) {
      log(SPCHAN_ERR, "Error WriteRead({} -> {}): {}", _fd, _srv_pn, res.error().String());
      return std::unexpected(res.error());
    }
  }
  return call;
}

std::expected<int, sigmaos::serr::Error> Channel::Close() {
  _closed = true;
  // Sanity check
  if (!_initialized) {
    return 0;
  }
  return _sp_clnt->CloseFD(_fd);
}

bool Channel::IsClosed() {
  return _closed;
}

bool Channel::IsInitialized() {
  return _initialized;
}

};
};
