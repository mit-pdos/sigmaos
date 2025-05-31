#include <rpc/spchannel/spchannel.h>

namespace sigmaos {
namespace rpc::spchannel {

bool Channel::_l = sigmaos::util::log::init_logger(SPCHAN);
bool Channel::_l_e = sigmaos::util::log::init_logger(SPCHAN_ERR);

// Initialize the channel & connect to the server
std::expected<int, sigmaos::serr::Error> Channel::Init() {
  // TODO: use consts
  std::string clone_pn = _srv_pn + "/rpc/clone";
  {
    auto res = _sp_clnt->GetFile(clone_pn);
    if (!res.has_value()) {
      log(SPCHAN_ERR, "Error GetFile clone file({}): {}", clone_pn, res.error().String());
      return std::unexpected(res.error());
    }
    _sid = *res.value();
  }
  {
    // TODO: use consts
    std::string sess_dev_pn = _srv_pn + "/rpc/" + _sid + "/data";
    auto res = _sp_clnt->Open(sess_dev_pn, sigmaos::sigmap::constants::ORDWR, false);
    if (!res.has_value()) {
      log(SPCHAN_ERR, "Error Open data file({}): {}", sess_dev_pn, res.error().String());
      return std::unexpected(res.error());
    }
    _fd = res.value();
  }
  log(SPCHAN, "Successfully initialized sigmap-based RPC channel to {}", _srv_pn);
  return 0;
}

std::expected<std::shared_ptr<sigmaos::io::transport::Call>, sigmaos::serr::Error> Channel::SendReceive(std::shared_ptr<sigmaos::io::transport::Call> call) {
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
  return _sp_clnt->CloseFD(_fd);
}

bool Channel::IsClosed() {
  return _closed;
}

};
};
