#include <proc/status.h>
#include <proxy/sigmap/proto/spproxy.pb.h>
#include <proxy/sigmap/sigmap.h>
#include <pybind11/pybind11.h>
#include <pybind11/stl.h>

#include <memory>
#include <string>

namespace py = pybind11;

std::unique_ptr<sigmaos::proxy::sigmap::Clnt> clnt;

void check_clnt() {
  if (!clnt) {
    throw std::runtime_error(
        "SigmaOS Client not initialized. Call splib.init_socket() first.");
  }
}

void init_clnt_if_needed() {
  if (!clnt) {
    clnt = std::make_unique<sigmaos::proxy::sigmap::Clnt>();
  }
}

template <typename T>
T unwrap(std::expected<T, sigmaos::serr::Error> result) {
  if (result.has_value()) {
    return result.value();
  } else {
    throw result.error();
  }
}

PYBIND11_MODULE(_clntlib, m) {
  m.doc() = "SigmaOS Python client library";

  PYBIND11_CONSTINIT static py::gil_safe_call_once_and_store<py::object>
      sigmaos_error_storage;
  sigmaos_error_storage.call_once_and_store_result(
      [&]() { return py::exception<sigmaos::serr::Error>(m, "SigmaOSError"); });
  py::register_exception_translator([](std::exception_ptr p) {
    try {
      if (p) std::rethrow_exception(p);
    } catch (const sigmaos::serr::Error& e) {
      py::set_error(sigmaos_error_storage.get_stored(), e.PyMessage().c_str());
    }
  });

  py::class_<TqidProto, py::smart_holder>(m, "Qid")
      .def(py::init<>())
      .def_property_readonly("type", &TqidProto::type)
      .def_property_readonly("version", &TqidProto::version)
      .def_property_readonly("path", &TqidProto::path);

  py::class_<TstatProto, py::smart_holder>(m, "Stat")
      .def(py::init<>())
      .def_property_readonly("type", &TstatProto::type)
      .def_property_readonly("dev", &TstatProto::dev)
      .def_property_readonly("qid", &TstatProto::qid)
      .def_property_readonly("mode", &TstatProto::mode)
      .def_property_readonly("atime", &TstatProto::atime)
      .def_property_readonly("mtime", &TstatProto::mtime)
      .def_property_readonly("length", &TstatProto::length)
      .def_property_readonly("name", &TstatProto::name)
      .def_property_readonly("uid", &TstatProto::uid)
      .def_property_readonly("gid", &TstatProto::gid)
      .def_property_readonly("muid", &TstatProto::muid);

  py::enum_<sigmaos::proc::Tstatus>(m, "Status")
      .value("Ok", sigmaos::proc::Tstatus::StatusOK)
      .value("Evicted", sigmaos::proc::Tstatus::StatusEvicted)
      .value("Error", sigmaos::proc::Tstatus::StatusErr)
      .value("Fatal", sigmaos::proc::Tstatus::StatusFatal)
      .value("Crash", sigmaos::proc::Tstatus::CRASH)
      .export_values();

  // ======== ProcClnt API ========
  m.def(
      "init_socket",
      []() { clnt = std::make_unique<sigmaos::proxy::sigmap::Clnt>(); },
      "Initialize the connection socket.");

  m.def(
      "started",
      []() {
        init_clnt_if_needed();
        unwrap(clnt->Started());
      },
      "Signal that the process has started.");

  m.def(
      "exited",
      [](const sigmaos::proc::Tstatus& status, const std::string& msg) {
        init_clnt_if_needed();
        std::string msg_copy = msg;
        unwrap(clnt->Exited(status, msg_copy));
      },
      "Signal that the process has exited.");

  m.def(
      "wait_evict",
      []() {
        init_clnt_if_needed();
        unwrap(clnt->WaitEvict());
      },
      "Wait for an eviction event.");

  // ============== SPProxy API Stubs =============
  m.def(
      "close",
      [](int fd) {
        init_clnt_if_needed();
        unwrap(clnt->CloseFD(fd));
      },
      "Close a file descriptor.", py::arg("fd"));

  m.def(
      "stat",
      [](const std::string& pn) {
        init_clnt_if_needed();
        auto stat = *unwrap(clnt->Stat(pn));
        return stat;
      },
      "Get file status.", py::arg("path"));

  m.def(
      "create",
      [](const std::string& pn, uint32_t perm, uint32_t mode) {
        init_clnt_if_needed();
        auto fd = unwrap(clnt->Create(pn, perm, mode));
        return fd;
      },
      "Create a file and return its file descriptor.", py::arg("path"),
      py::arg("perm"), py::arg("mode"));

  m.def(
      "open",
      [](const std::string& pn, uint32_t mode, bool wait) {
        init_clnt_if_needed();
        auto fd = unwrap(clnt->Open(pn, mode, wait));
        return fd;
      },
      "Open a file and return its file descriptor.", py::arg("path"),
      py::arg("mode"), py::arg("wait"));

  m.def(
      "rename",
      [](const std::string& src, const std::string& dst) {
        init_clnt_if_needed();
        unwrap(clnt->Rename(src, dst));
      },
      "Rename a file from src to dst.", py::arg("src"), py::arg("dst"));

  m.def(
      "remove",
      [](const std::string& pn) {
        init_clnt_if_needed();
        unwrap(clnt->Remove(pn));
      },
      "Remove a file.", py::arg("path"));

  m.def(
      "get_file",
      [](const std::string& pn) {
        init_clnt_if_needed();
        auto contents = unwrap(clnt->GetFile(pn));
        return py::bytes(*contents);
      },
      "Get the contents of a file.", py::arg("path"));

  m.def(
      "put_file",
      [](const std::string& pn, uint32_t perm, uint32_t mode,
         const std::string& data, uint64_t offset, uint64_t leaseID) {
        init_clnt_if_needed();
        std::string d = data;
        auto size = unwrap(clnt->PutFile(pn, perm, mode, &d, offset, leaseID));
        return size;
      },
      "Put data into a file, returns number of bytes written.", py::arg("path"),
      py::arg("perm"), py::arg("mode"), py::arg("data"), py::arg("offset"),
      py::arg("leaseID"));

  m.def(
      "read",
      [](int fd, size_t len) {
        init_clnt_if_needed();
        std::string buffer(len, '\0');
        auto size = unwrap(clnt->Read(fd, &buffer));
        buffer.resize(size);
        return py::bytes(buffer);
      },
      "Read the specified number of bytes from a file descriptor.",
      py::arg("fd"), py::arg("len"));

  m.def(
      "pread",
      [](int fd, size_t len, uint64_t offset) {
        init_clnt_if_needed();
        std::string buffer(len, '\0');
        sigmaos::sigmap::types::Toffset off = offset;
        auto size = unwrap(clnt->Pread(fd, &buffer, off));
        buffer.resize(size);
        return py::bytes(buffer);
      },
      "Read the specified number of bytes from a file descriptor at a given "
      "offset.",
      py::arg("fd"), py::arg("len"), py::arg("offset"));

  m.def(
      "write",
      [](int fd, const std::string& bytes) {
        init_clnt_if_needed();
        std::string b = bytes;
        auto written = unwrap(clnt->Write(fd, &b));
        return written;
      },
      "Write bytes to a file descriptor, returns number of bytes written.",
      py::arg("fd"), py::arg("bytes"));

  m.def(
      "seek",
      [](int fd, uint64_t offset) {
        init_clnt_if_needed();
        unwrap(clnt->Seek(fd, offset));
      },
      "Seek to a specific offset in a file descriptor.", py::arg("fd"),
      py::arg("offset"));

  m.def(
      "clnt_id",
      []() {
        init_clnt_if_needed();
        return unwrap(clnt->ClntID());
      },
      "Get the client ID.");
  // TODO: everything after Seek in cpp/proxy/sigmap/sigmap.h
  // TODO: proper handling of std::expected
  //       have functions raise instead of returning -1
  //       also: check if clnt is initialized
}