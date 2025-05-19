#include <threadpool/threadpool.h>

namespace sigmaos {
namespace threadpool {

bool Threadpool::_l = sigmaos::util::log::init_logger(THREADPOOL);
bool Threadpool::_l_e = sigmaos::util::log::init_logger(THREADPOOL_ERR);

void Threadpool::Run(std::function<void(void)> f) {
  {
    std::lock_guard<std::mutex> guard(_mu);
    // If there are no idle threads, create a new thread.
    if (_n_idle == 0) {
      add_thread();
    }
    // Add to the work queue
    _work_q.push(f);
  }
  _cond.notify_one();
}

// Thread main loop
void Threadpool::work() {
  while (true) {
    bool wakeup_next_waiter = false;
    std::function<void(void)> f;
    {
      // Wait until there is work in the queue
      std::unique_lock lk(_mu);
      _cond.wait(lk, [this]{ return _work_q.size() > 0; });
      _n_idle--;
      // Claim the next work queue item
      f = _work_q.front();
      _work_q.pop();
      // If there is still work to be done in the work queue, wake up the next
      // waiter
      wakeup_next_waiter = _work_q.size() > 0;
    }
    // Wake up the next waiter if there is work left in the queue
    if (wakeup_next_waiter) {
      _cond.notify_one();
    }
    // Do the work
    f();
    // Mark self as idle
    {
      std::lock_guard<std::mutex> guard(_mu);
      _n_idle++;
    }
  }
}

// Add a new thread to the thread pool. Caller holds lock.
void Threadpool::add_thread() {
  log(THREADPOOL, "{} add_thread", _name);
  // Add a new thread to the pool
  _threads.push_back(std::thread(std::bind(&Threadpool::work, this)));
  // Increment the number of idle threads
  _n_idle++;
}

};
};
