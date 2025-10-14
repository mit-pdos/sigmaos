#include <stdio.h>
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

extern "C" {
    __attribute__((export_name("entrypoint")))
    int entrypoint() {
        int res = Started();
        return res;
    }
}

int main() {
    return 0;
}
