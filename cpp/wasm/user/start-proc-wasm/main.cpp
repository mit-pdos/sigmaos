#include <stdio.h>

extern "C" {
    __attribute__((import_module("env"), import_name("Started")))
    int Started();
    
    __attribute__((import_module("env"), import_name("Exited")))
    void Exited(int status);
}

extern "C" {
    __attribute__((export_name("test")))
    int test(int i) {
        int started = Started();
        Exited(started * 2);
        return i + 2;
    }
}

int main() {
    printf("hello world");
    return 0;
}
