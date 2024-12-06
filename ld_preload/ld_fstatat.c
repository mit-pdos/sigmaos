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
        read(sfd, x2, 2);

        init_done = 1;
    }
    
    const char* prefix = "/~~";
    int i = 0;
    while(filename[i] != 0 && i < 3) {
        if (filename[i] != prefix[i]) {
            return filename;
        }
        i++;
    }
    
    if (i < 3) return filename;

    fflush(stdout);
    char* x = malloc(512 * sizeof(char));
    sprintf(x, "%s%s", "/tmp/python", &(filename[3]));

    write(sfd, "pf", 2);
    write(sfd, &(filename[3]), strlen(filename) - 3);
    write(sfd, "\n", 1);
    printf("LD_PRELOAD: wrote to socket: %s\n", filename);
    read(sfd, x2, 1);
    while(x2[0] != 'd') {
        read(sfd, x2, 1);
    }
    printf("LD_PRELOAD: finished\n");
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
