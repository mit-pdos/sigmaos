#define _GNU_SOURCE
#include <sys/stat.h>
#include <sys/socket.h>
#include <sys/un.h>
#include <stdio.h>
#include <stdlib.h>
#include <stdarg.h>
#include <dlfcn.h>
#include <dirent.h>
#include <string.h>
#include <unistd.h>
#include <errno.h>

const char* socketEnvVar = "SIGMA_PYPROXY_FD";
int sfd = 0;
int init_done = 0;

const char* get_path(const char *filename)
{
    char x2[3];

    if(!init_done) {
        const char* sfd_str = getenv(socketEnvVar);
        if (sfd_str == NULL) {
          exit(-1);
        }
        sfd = atoi(sfd_str);

        write(sfd, "pb", 2);
        write(sfd, "\n", 1);
        read(sfd, x2, 1);
        while(x2[0] != 'd') {
            read(sfd, x2, 1);
        }

        init_done = 1;
    }

    // Python's initial call to obtain all present libraries
    if (strcmp("/~~/Lib", filename) == 0) { // Figure out why this is still necessary?
        return "/tmp/python/superlib";
    }
    if (strcmp("/tmp/python/Lib", filename) == 0) {
        return "/tmp/python/superlib";
    }

    // Catch /~~ prefix (only used by Python files -- not libs!)
    const char* prefix = "/~~";
    int i = 0;
    while (filename[i] != 0 && i < 3) {
        if (filename[i] != prefix[i]) {
            break;
        }
        i++;
    }

    // Catch /tmp/python/Lib prefix
    const char* pathPrefix = "/tmp/python/Lib";
    int j = 0;
    while (filename[j] != 0 && j < 15) {
        if (filename[j] != pathPrefix[j]) {
            break;
        }
        j++;
    }
    
    if (i != 3 && j != 15) return filename;

    fflush(stdout);
    char* x = malloc(512 * sizeof(char));

    if (i == 3) {
        sprintf(x, "%s%s", "/tmp/python", &(filename[3]));
    } else {
        sprintf(x, "%s", filename);
    }

    write(sfd, "pf", 2);
    if (i == 3) {
        write(sfd, &(filename[3]), strlen(filename) - 3);
    } else {
        write(sfd, &(filename[11]), strlen(filename) - 11);
    }
    write(sfd, "\n", 1);
    read(sfd, x2, 1);
    while(x2[0] != 'd') {
        read(sfd, x2, 1);
    }
    return x;
}

int stat(const char *path, struct stat *buf) {
    static int (*stat_func)(const char*, struct stat*) = NULL;
    stat_func = (int(*)(const char*, struct stat*)) dlsym(RTLD_NEXT, "stat");
    int res = stat_func(get_path(path), buf);
    return res;
}

int fstat(int fd, struct stat *st)
{
    static int (*fstat_func)(int, struct stat*) = NULL;
    fstat_func = (int(*)(int, struct stat*)) dlsym(RTLD_NEXT, "fstat");
    int res = fstat_func(fd, st);
    return res;
}

int open(const char *filename, int flags, mode_t mode)
{
    static int (*open_func)(const char*, int, mode_t) = NULL;
    open_func = (int(*)(const char*, int, mode_t)) dlsym(RTLD_NEXT, "open");
    int res = open_func(get_path(filename), flags, mode);
    return res;
}

FILE * fopen( const char * filename,
              const char * mode )
{
    static FILE * (*fopen_func)(const char*, const char*) = NULL;
    fopen_func = (FILE* (*)(const char*, const char*)) dlsym(RTLD_NEXT, "fopen");
    FILE * res = fopen_func(get_path(filename), mode);
    return res;
}

int openat(int dirfd, const char *pathname, int flags, mode_t mode)
{
    static int (*open_func)(int, const char*, int, mode_t) = NULL;
    open_func = (int(*)(int, const char*, int, mode_t)) dlsym(RTLD_NEXT, "openat");
    int res = open_func(dirfd, get_path(pathname), flags, mode);
    return res;
}

DIR * opendir(const char* name)
{
    static DIR * (*opendir_func)(const char*) = NULL;
    opendir_func = (DIR*(*)(const char*)) dlsym(RTLD_NEXT, "opendir");
    DIR* res = opendir_func(get_path(name));
    return res;
}

int newfstatat(int dirfd, const char *pathname, struct stat *statbuf, int flags) {
    static int (*newfstatat_func)(int, const char *, struct stat *, int) = NULL;
    newfstatat_func = (int (*)(int, const char *, struct stat *, int)) dlsym(RTLD_NEXT, "newfstatat");
    int res = newfstatat_func(dirfd, get_path(pathname), statbuf, flags);
    return res;
}

int fstatat(int dirfd, const char *pathname, struct stat *statbuf, int flags) {
    static int (*fstatat_func)(int, const char *, struct stat *, int) = NULL;
    fstatat_func = (int (*)(int, const char *, struct stat *, int)) dlsym(RTLD_NEXT, "fstatat");
    int res = fstatat_func(dirfd, get_path(pathname), statbuf, flags);
    return res;
}

int lstat(const char *pathname, struct stat *statbuf) {
    static int (*lstat_func)(const char *, struct stat *) = NULL;
    lstat_func = (int (*)(const char *, struct stat *)) dlsym(RTLD_NEXT, "lstat");
    int res = lstat_func(get_path(pathname), statbuf);
    return res;
}

ssize_t readlink(const char *pathname, char *buf, size_t bufsiz) {
    static ssize_t (*readlink_func)(const char *, char *, size_t) = NULL;
    readlink_func = (ssize_t (*)(const char *, char *, size_t)) dlsym(RTLD_NEXT, "readlink");
    ssize_t res = readlink_func(get_path(pathname), buf, bufsiz);
    return res;
}

int creat(const char *pathname, mode_t mode) {
    static int (*creat_func)(const char *, mode_t) = NULL;
    creat_func = (int (*)(const char *, mode_t)) dlsym(RTLD_NEXT, "creat");
    int res = creat_func(get_path(pathname), mode);
    return res;
}

FILE *fopen64(const char *pathname, const char *mode) {
    static FILE *(*fopen64_func)(const char *, const char *) = NULL;
    fopen64_func = (FILE* (*)(const char *, const char *)) dlsym(RTLD_NEXT, "fopen64");
    FILE * res = fopen64_func(get_path(pathname), mode);
    return res;
}

int open64(const char *pathname, int flags, mode_t mode) {
    static int (*open64_func)(const char *, int, mode_t) = NULL;
    open64_func = (int (*)(const char *, int, mode_t)) dlsym(RTLD_NEXT, "open64");
    int res = open64_func(get_path(pathname), flags, mode);
    return res;
}

int stat64(const char *pathname, struct stat64 *statbuf) {
    static int (*stat64_func)(const char *, struct stat64 *) = NULL;
    stat64_func = (int (*)(const char *, struct stat64 *)) dlsym(RTLD_NEXT, "stat64");
    int res = stat64_func(get_path(pathname), statbuf);
    return res;
}
