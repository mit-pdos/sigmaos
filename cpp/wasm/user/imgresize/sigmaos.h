#ifndef SIGMAOS_H
#define SIGMAOS_H

#include <stdint.h>

// SigmaOS host function imports
extern "C" {
    // Process lifecycle
    __attribute__((import_module("env"), import_name("Started")))
    int32_t Started();

    __attribute__((import_module("env"), import_name("Exited")))
    void Exited(int32_t status);

    // File I/O
    __attribute__((import_module("env"), import_name("Open")))
    int32_t Open(const char* path_ptr, int32_t path_len, int32_t mode);

    __attribute__((import_module("env"), import_name("Create")))
    int32_t Create(const char* path_ptr, int32_t path_len, int32_t perm, int32_t mode);

    __attribute__((import_module("env"), import_name("Read")))
    int32_t Read(int32_t fd, uint8_t* buf_ptr, int32_t buf_len);

    __attribute__((import_module("env"), import_name("Write")))
    int32_t Write(int32_t fd, const uint8_t* buf_ptr, int32_t buf_len);

    __attribute__((import_module("env"), import_name("CloseFd")))
    int32_t CloseFd(int32_t fd);

    // Arguments
    __attribute__((import_module("env"), import_name("GetArgc")))
    int32_t GetArgc();

    __attribute__((import_module("env"), import_name("GetArgvLen")))
    int32_t GetArgvLen(int32_t idx);

    __attribute__((import_module("env"), import_name("GetArgs")))
    int32_t GetArgs(int32_t idx, char* buf_ptr, int32_t buf_len);

    // Logging
    __attribute__((import_module("env"), import_name("Log")))
    void Log(const char* msg_ptr, int32_t msg_len);
}

#define OREAD   0x0
#define OWRITE  0x1
#define ORDWR   0x2
#define OAPPEND 0x8

#endif // SIGMAOS_H
