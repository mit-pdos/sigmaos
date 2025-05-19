#pragma once

#include <memory>
#include <mutex>
#include <condition_variable>
#include <thread>
#include <vector>
#include <queue>
#include <expected>
#include <format>

#include <util/log/log.h>
#include <serr/serr.h>
#include <sigmap/sigmap.pb.h>
#include <sigmap/const.h>
#include <proxy/sigmap/sigmap.h>

namespace sigmaos {
namespace threadpool {

const std::string THREADPOOL = "THREADPOOL";
const std::string THREADPOOL_ERR = THREADPOOL + sigmaos::util::log::ERR;

class Threadpool {
  public:
  Threadpool() : Threadpool(0) {}
  Threadpool(int n_initial_threads) : _mu(), _cond(), _n_idle(0), _threads(), _work_q() {
    for (int i = 0; i < n_initial_threads; i++) {
      add_thread();
    }
  }
  ~Threadpool() {}

  // Run a function in the threadpool
  void Run(std::function<void(void)> f);

  private:
  std::mutex _mu;
  std::condition_variable _cond;
  int _n_idle;
  std::vector<std::thread> _threads;
  std::queue<std::function<void(void)>> _work_q;
  // Used for logger initialization
  static bool _l;
  static bool _l_e;

  // Start a new thread
  void add_thread();
  // Thread main loop
  void work();
};

};
};
