message(STATUS "Building python clntlib library")

add_library(clntlib SHARED clntlib.cc) 

target_link_libraries(clntlib PRIVATE sigmap_proxy)
