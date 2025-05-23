cmake_minimum_required(VERSION 3.28)
project(sigmaos CXX)

set(CMAKE_CXX_STANDARD 23)
set(CMAKE_CXX_STANDARD_REQUIRED True)
set(CMAKE_CXX_STANDARD_REQUIRED ON)

if(NOT CMAKE_BUILD_TYPE)
  set(CMAKE_BUILD_TYPE Release)
endif()

# External packages
find_package(Protobuf REQUIRED)
find_package(absl REQUIRED)
find_package(spdlog REQUIRED)

# SigmaOS packages
add_subdirectory(serr)
add_subdirectory(io)
add_subdirectory(util)
add_subdirectory(proc)
add_subdirectory(proxy)
add_subdirectory(sigmap)
add_subdirectory(rpc)
add_subdirectory(apps)
add_subdirectory(threadpool)

# Procs
add_subdirectory(user)
