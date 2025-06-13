#include <rpc/srv.h>

#include <rpc/blob.h>

namespace sigmaos {
namespace rpc::srv {

bool Srv::_l = sigmaos::util::log::init_logger(RPCSRV);
bool Srv::_l_e = sigmaos::util::log::init_logger(RPCSRV_ERR);

std::expected<std::shared_ptr<sigmaos::io::transport::Call>, sigmaos::serr::Error> Srv::serve_request(std::shared_ptr<sigmaos::io::transport::Call> req) {
  log(RPCSRV, "rpc::Srv::serve_request");
  Rerror err;
  std::shared_ptr<sigmaos::io::transport::Call> rep;
  Rep wrapper_rep;
  wrapper_rep.set_allocated_err(&err);
  auto res = unwrap_and_run_rpc(req);
  if (!res.has_value()) {
    auto rerr = res.error();
    err.set_errcode(rerr.GetError());
    err.set_obj(rerr.GetMsg());
    // Reuse the request input for output if an error occurred
    rep = req;
    // In the event of an error, no output buffers will be appended, so we need
    // to append one for the wrapper
    rep->GetOutIOVec()->AddBuffers(2);
  } else {
    rep = res.value();
    err.set_errcode(sigmaos::serr::TErrNoError);
  }
  auto serialized_wrapper_rep_buf = rep->GetOutIOVec()->GetBuffer(0)->Get();
  wrapper_rep.SerializeToString(serialized_wrapper_rep_buf);
  // Release err since its memory is owned by the stack of this function call.
  {
    auto _ = wrapper_rep.release_err();
  }
  return rep;
}

std::expected<std::shared_ptr<sigmaos::io::transport::Call>, sigmaos::serr::Error> Srv::unwrap_and_run_rpc(std::shared_ptr<sigmaos::io::transport::Call> req) {
  auto err = new Rerror();
  auto in_iov = req->GetInIOVec();
  Req wrapper_req;
  // Deserialize the wrapper
  auto wrapper_req_buf = in_iov->GetBuffer(0);
  wrapper_req.ParseFromString(*(wrapper_req_buf->Get()));
  // Remove the wrapper from the out IOVec
  in_iov->RemoveBuffer(0);
  if (!_rpc_endpoints.contains(wrapper_req.method())) {
    log(RPCSRV_ERR, "rpc::Srv unknown endpoint: {}", wrapper_req.method());
    return std::unexpected(sigmaos::serr::Error(sigmaos::serr::Terror::TErrError, std::format("Invalid method %v", wrapper_req.method())));
  }
  auto rpce = _rpc_endpoints.at(wrapper_req.method());
  // Parse the RPC input protobuf
  auto req_proto = rpce->GetInput();
  req_proto->ParseFromString(*(in_iov->GetBuffer(0)->Get()));
  // Set the request's blob's IOV to point to the input data, if applicable.
  set_blob_iov(in_iov, *req_proto);
  auto rep_proto = rpce->GetOutput();
  auto start = GetCurrentTime();
  auto res = rpce->GetFunction()(req_proto, rep_proto);
  if (!res.has_value()) {
    log(RPCSRV_ERR, "rpc::Srv unwrap_and_run_req Run function: {}", res.error());
    return std::unexpected(res.error());
  }
  log(RPCSRV, "External RPC stub {} latency={:0.3f}ms", wrapper_req.method(), LatencyMS(start));
  auto out_iov = req->GetOutIOVec();
  // Extract any input IOVecs from the reply RPC
  extract_blob_iov(*rep_proto, out_iov);
  out_iov->AddBuffers(2);
  auto serialized_rep_buf = out_iov->GetBuffer(1);
  // Serialize the reply 
  rep_proto->SerializeToString(serialized_rep_buf->Get());
  return req;
}

std::shared_ptr<TendpointProto> Srv::GetEndpoint() {
  auto ep = std::make_shared<TendpointProto>();
  auto addr = ep->add_addr();
  addr->set_ipstr(_sp_clnt->ProcEnv()->GetOuterContainerIP());
  addr->set_portint(_netsrv->GetPort());
  ep->set_type(sigmaos::sigmap::constants::EXTERNAL_EP);
  return ep;
}

void Srv::ExposeRPCHandler(std::shared_ptr<RPCEndpoint> rpce) {
  if (_rpc_endpoints.contains(rpce->GetMethod())) {
    fatal("Double-register method {}", rpce->GetMethod());
  }
  _rpc_endpoints[rpce->GetMethod()] = rpce;
}

std::expected<int, sigmaos::serr::Error> Srv::RegisterEP(std::string pn) {
  auto start = GetCurrentTime();
  {
    auto ep = GetEndpoint();
    auto res = _sp_clnt->RegisterEP(pn, ep);
    if (!res.has_value()) {
      log(RPCSRV_ERR, "Error RegisterEP: {}", res.error());
      return std::unexpected(res.error());
    }
    log(RPCSRV, "Registered sigmaEP {} in realm at pn {}", ep->ShortDebugString(), pn);
  }
  LogSpawnLatency(_sp_clnt->ProcEnv()->GetPID(), _sp_clnt->ProcEnv()->GetSpawnTime(), start, "RegisterEP");
  return 0;
}

[[noreturn]] void Srv::Run() {
  // Mark server as started
  {
    auto start = GetCurrentTime();
    auto res = _sp_clnt->Started();
    if (!res.has_value()) {
      log(RPCSRV, "Error Started: {}", res.error());
    }
    LogSpawnLatency(_sp_clnt->ProcEnv()->GetPID(), _sp_clnt->ProcEnv()->GetSpawnTime(), start, "spproxyclnt.Started");
  }
  log(RPCSRV, "Started");
  // Start a new thread to wait for the eviction signal
  auto evict_thread = _sp_clnt->StartWaitEvictThread();
  // Join the thread
  evict_thread.join();
  log(RPCSRV, "Evicted");
  // Mark server as exited
  {
    std::string msg = "Evicted! Done serving.";
    auto res = _sp_clnt->Exited(sigmaos::proc::Tstatus::StatusEvicted, msg);
    if (!res.has_value()) {
      log(RPCSRV_ERR, "Error exited: {}", res.error());
    }
  }
  log(RPCSRV, "Exited");
  // exit
  std::exit(0);
}

};
};
