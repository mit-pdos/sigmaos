message(STATUS "Building sigmaos Python C shim")

add_library(sigmaos_py SHARED sigmaos_c.cc)
target_link_libraries(sigmaos_py PUBLIC sigmap_proxy s3_proxy ux_proxy io_conn_tcp)
target_include_directories(sigmaos_py PUBLIC ${CMAKE_SOURCE_DIR})
