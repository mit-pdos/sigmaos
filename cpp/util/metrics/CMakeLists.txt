add_library(util_metrics
    server_metrics.cc
)

target_include_directories(util_metrics PUBLIC
    ${CMAKE_SOURCE_DIR}
)
