#pragma once

#include <memory>
#include <vector>

namespace sigmaos {
namespace io::iovec {

// Abstraction over a memory buffer used for I/O. If the memory buffer comes
// from a protobuf, it is likely owned by a caller higher up in the stack. So,
// don't reference count the buffer (to avoid early/unwanted deletion).
// Otherwise, reference count it.
class Buffer {
 public:
  Buffer(std::string *ptr)
      : _ref_counted(false), _non_rc_ptr(ptr), _rc_ptr(nullptr) {}
  Buffer(std::shared_ptr<std::string> ptr)
      : _ref_counted(true), _non_rc_ptr(nullptr), _rc_ptr(ptr) {}
  ~Buffer() {}

  std::string *Get() {
    if (_ref_counted) {
      return _rc_ptr.get();
    } else {
      return _non_rc_ptr;
    }
  }

  int Size() {
    if (_ref_counted) {
      return _rc_ptr->size();
    } else {
      return _non_rc_ptr->size();
    }
  }

  bool IsRefCounted() { return _ref_counted; }

 private:
  bool _ref_counted;
  std::string *_non_rc_ptr;
  std::shared_ptr<std::string> _rc_ptr;
};

class IOVec {
 public:
  IOVec() : _bufs() {}
  IOVec(int n) : _bufs(n) {}
  ~IOVec() {}

  std::shared_ptr<Buffer> GetBuffer(int idx) { return _bufs.at(idx); }
  void SetBuffer(int idx, std::shared_ptr<Buffer> buf) { _bufs.at(idx) = buf; }
  void AppendBuffer(std::shared_ptr<Buffer> buf) { _bufs.push_back(buf); }
  void AddBuffers(int n) {
    for (int i = 0; i < n; i++) {
      AppendBuffer(std::make_shared<Buffer>(std::make_shared<std::string>()));
    }
  }
  std::shared_ptr<Buffer> RemoveBuffer(int idx) {
    auto buf = GetBuffer(idx);
    _bufs.erase(_bufs.begin() + idx);
    return buf;
  }
  int Size() { return _bufs.size(); }
  void Resize(int newsz) { _bufs.resize(newsz); }

 private:
  std::vector<std::shared_ptr<Buffer>> _bufs;
};

};  // namespace io::iovec
};  // namespace sigmaos
