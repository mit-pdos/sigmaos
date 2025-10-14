#include "sigmaos.h"
#include <string.h>
#include <stdio.h>
#include <stdlib.h>
#include <time.h>

void log(const char* msg) {
    Log(msg, strlen(msg));
}

// Get current time in microseconds
long long get_time_usec() {
    struct timespec ts;
    clock_gettime(CLOCK_MONOTONIC, &ts);
    return (long long)ts.tv_sec * 1000000LL + ts.tv_nsec / 1000LL;
}

bool getArg(int idx, char* buf, int buf_size) {
    int len = GetArgvLen(idx);
    if (len < 0 || len >= buf_size) {
        return false;
    }
    if (GetArgs(idx, buf, len + 1) < 0) {
        return false;
    }
    buf[len] = '\0';
    return true;
}

#define STATUS_OK 0
#define STATUS_ERR 1

extern "C" {
    __attribute__((export_name("entrypoint")))
    int entrypoint() {
        long long t_start = get_time_usec();
        char msg[256];

        log("[imgresize] Entering imgresize()");

        long long t_before_started = get_time_usec();
        if (Started() < 0) {
            log("Failed to call Started()");
            return STATUS_ERR;
        }
        long long t_after_started = get_time_usec();
        snprintf(msg, sizeof(msg), "[imgresize] Started() took %lld usec",
                 t_after_started - t_before_started);
        log(msg);

        int argc = GetArgc();
        snprintf(msg, sizeof(msg), "[imgresize] Started with %d args", argc);
        log(msg);

        if (argc < 3) {
            log("ERROR: Expected at least 3 arguments: input output nrounds");
            return STATUS_ERR;
        }

        char input_path[256];
        char output_path[256];
        char nrounds_str[32];

        if (!getArg(0, input_path, sizeof(input_path))) {
            log("ERROR: Failed to get input path");
            return STATUS_ERR;
        }

        if (!getArg(1, output_path, sizeof(output_path))) {
            log("ERROR: Failed to get output path");
            return STATUS_ERR;
        }

        if (!getArg(2, nrounds_str, sizeof(nrounds_str))) {
            log("ERROR: Failed to get nrounds");
            return STATUS_ERR;
        }

        int nrounds = atoi(nrounds_str);

        snprintf(msg, sizeof(msg), "[imgresize] Processing: %s -> %s (rounds=%d)",
                 input_path, output_path, nrounds);
        log(msg);

        // Open input file
        long long t_open_start = get_time_usec();
        int input_fd = Open(input_path, strlen(input_path), OREAD);
        long long t_open_end = get_time_usec();

        if (input_fd < 0) {
            snprintf(msg, sizeof(msg), "[imgresize] ERROR: Failed to open input file: %s", input_path);
            log(msg);
            return STATUS_ERR;
        }

        snprintf(msg, sizeof(msg), "[imgresize] Opened input file fd=%d (took %lld usec)",
                 input_fd, t_open_end - t_open_start);
        log(msg);

        // Allocate buffer
        const int CHUNK_SIZE = 64 * 1024;
        uint8_t* buffer = (uint8_t*)malloc(CHUNK_SIZE);
        if (!buffer) {
            log("[imgresize] ERROR: Failed to allocate buffer");
            CloseFd(input_fd);
            return STATUS_ERR;
        }

        // Read input file
        long long t_read_start = get_time_usec();
        int total_read = 0;
        int bytes_read;

        while ((bytes_read = Read(input_fd, buffer, CHUNK_SIZE)) > 0) {
            total_read += bytes_read;
        }
        long long t_read_end = get_time_usec();

        snprintf(msg, sizeof(msg), "[imgresize] Read %d bytes from input (took %lld usec, %lld MB/s)",
                 total_read, t_read_end - t_read_start,
                 (total_read * 1000000LL) / ((t_read_end - t_read_start) * 1024 * 1024 + 1));
        log(msg);

        CloseFd(input_fd);

        // Simulate image processing (decode, resize nrounds times, encode)
        long long t_process_start = get_time_usec();
        log("[imgresize] Processing image (decode/resize/encode)...");
        // TODO: Add actual JPEG decode, resize, encode here
        long long t_process_end = get_time_usec();

        snprintf(msg, sizeof(msg), "[imgresize] Processing took %lld usec", t_process_end - t_process_start);
        log(msg);

        // Create output file
        long long t_create_start = get_time_usec();
        int output_fd = Create(output_path, strlen(output_path), 0777, OWRITE);
        long long t_create_end = get_time_usec();

        if (output_fd < 0) {
            snprintf(msg, sizeof(msg), "[imgresize] ERROR: Failed to create output file: %s", output_path);
            log(msg);
            free(buffer);
            return STATUS_ERR;
        }

        snprintf(msg, sizeof(msg), "[imgresize] Created output file fd=%d (took %lld usec)",
                 output_fd, t_create_end - t_create_start);
        log(msg);

        // Write output
        long long t_write_start = get_time_usec();
        const char* placeholder = "PLACEHOLDER_RESIZED_IMAGE_DATA";
        int bytes_written = Write(output_fd, (const uint8_t*)placeholder, strlen(placeholder));
        long long t_write_end = get_time_usec();

        snprintf(msg, sizeof(msg), "[imgresize] Wrote %d bytes to output (took %lld usec)",
                 bytes_written, t_write_end - t_write_start);
        log(msg);

        CloseFd(output_fd);
        free(buffer);

        long long t_end = get_time_usec();
        snprintf(msg, sizeof(msg), "[imgresize] Complete! Total time: %lld usec", t_end - t_start);
        log(msg);

        return STATUS_OK;
    }
}

int main() {
    return 0;
}
