#pragma once

#include <memory>
#include <vector>
#include <expected>
#include <format>

#include <google/protobuf/util/time_util.h>

#include <util/log/log.h>
#include <util/perf/perf.h>
#include <io/net/srv.h>
#include <io/conn/conn.h>
#include <io/transport/transport.h>
#include <io/conn/tcp/tcp.h>
#include <io/demux/srv.h>
#include <serr/serr.h>
#include <sigmap/sigmap.pb.h>
#include <sigmap/const.h>
#include <proxy/sigmap/sigmap.h>

namespace sigmaos {
namespace rpc::srv {

const std::string RPCSRV = "RPCSRV";
const std::string RPCSRV_ERR = RPCSRV + sigmaos::util::log::ERR;

typedef std::function<std::expected<int, sigmaos::serr::Error>(std::shared_ptr<google::protobuf::Message>, std::shared_ptr<google::protobuf::Message>)> RPCFunction;

class RPCEndpoint {
  public:
  RPCEndpoint(std::string method, std::shared_ptr<google::protobuf::Message> input, std::shared_ptr<google::protobuf::Message> output, RPCFunction fn) : _method(method), _input(input), _output(output), _fn(fn) {}
  ~RPCEndpoint() {}

  // Construct & return a new input type
  std::shared_ptr<google::protobuf::Message> GetInput() { return std::shared_ptr<google::protobuf::Message>(_input->New()); }
  // Construct & return a new output type
  std::shared_ptr<google::protobuf::Message> GetOutput() { return std::shared_ptr<google::protobuf::Message>(_output->New()); }
  RPCFunction GetFunction() { return _fn; }
  std::string GetMethod() { return _method; }

  private:
  std::string _method;
  std::shared_ptr<google::protobuf::Message> _input;
  std::shared_ptr<google::protobuf::Message> _output;
  RPCFunction _fn;
};

class Srv {
  public:
  Srv(std::shared_ptr<sigmaos::proxy::sigmap::Clnt> sp_clnt) : Srv(sp_clnt, 0) {}
  Srv(std::shared_ptr<sigmaos::proxy::sigmap::Clnt> sp_clnt, int demux_init_nthread) : _done(false), _sp_clnt(sp_clnt), _rpc_endpoints() {
    log(RPCSRV, "Starting net server");
    auto start = GetCurrentTime();
    _netsrv = std::make_shared<sigmaos::io::net::Srv>("tcpsrv", std::bind(&Srv::serve_request, this, std::placeholders::_1), demux_init_nthread);
    LogSpawnLatency(_sp_clnt->ProcEnv()->GetPID(), _sp_clnt->ProcEnv()->GetSpawnTime(), start, "Make NetSrv");
    int port = _netsrv->GetPort();
    log(RPCSRV, "Net server started with port {}", port);
  }
  ~Srv() {}

  // Expose an RPC handler via this RPC server
  void ExposeRPCHandler(std::shared_ptr<RPCEndpoint> rpce);
  // Register this server's SigmaEP in the realm namespace at path pn
  std::expected<int, sigmaos::serr::Error> RegisterEP(std::string pn);
  std::shared_ptr<TendpointProto> GetEndpoint();
  std::expected<int, sigmaos::serr::Error> Close() {
    _done = true;
    return _netsrv->Close();
  }

  // Mark self as started, run, block until evicted, and exit once evicted.
  [[noreturn]] void Run();

  private:
  bool _done;
  std::shared_ptr<sigmaos::proxy::sigmap::Clnt> _sp_clnt;
  std::shared_ptr<sigmaos::io::net::Srv> _netsrv;
  std::map<std::string, std::shared_ptr<RPCEndpoint>> _rpc_endpoints;
  // Used for logger initialization
  static bool _l;
  static bool _l_e;
  
  std::expected<std::shared_ptr<sigmaos::io::transport::Call>, sigmaos::serr::Error> serve_request(std::shared_ptr<sigmaos::io::transport::Call> req);
  std::expected<std::shared_ptr<sigmaos::io::transport::Call>, sigmaos::serr::Error> unwrap_and_run_rpc(std::shared_ptr<sigmaos::io::transport::Call> req);
};

};
};
