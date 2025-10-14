#include <stdio.h>
#include <time.h>
#include <string.h>

extern "C" {
    __attribute__((import_module("env"), import_name("Started")))
    int Started();

    __attribute__((import_module("env"), import_name("Exited")))
    void Exited(int status);

    __attribute__((import_module("env"), import_name("Log")))
    void Log(const char* msg_ptr, int msg_len);
}

void log(const char* msg) {
    Log(msg, strlen(msg));
}

long long get_time_usec() {
    struct timespec ts;
    clock_gettime(CLOCK_MONOTONIC, &ts);
    return (long long)ts.tv_sec * 1000000LL + ts.tv_nsec / 1000LL;
}

extern "C" {
    __attribute__((export_name("entrypoint")))
    int entrypoint() {
        long long start_time = get_time_usec();

        log("[spawn-latency-wasm] Entering start function");

        long long before_started = get_time_usec();
        int started = Started();
        long long after_started = get_time_usec();

        char msg[256];
        snprintf(msg, sizeof(msg), "[spawn-latency-wasm] Started() call took %lld usec, returned %d",
                 after_started - before_started, started);
        log(msg);


        long long end_time = get_time_usec();
        snprintf(msg, sizeof(msg), "[spawn-latency-wasm] Total execution time: %lld usec, started=%d",
                 end_time - start_time, started);
        log(msg);

        return started;
    }
}

int main() {
    log("[spawn-latency-wasm] main() called");
    return 0;
}
