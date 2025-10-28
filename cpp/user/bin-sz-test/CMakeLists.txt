message(STATUS "Building bin-sz-test proc")

add_compile_options(-Os)

set(CMAKE_INTERPROCEDURAL_OPTIMIZATION ON)

add_link_options(-Wl,--gc-sections -Wl,--strip-all)

add_executable(bin-sz-test
  main.cc
)

target_link_libraries(bin-sz-test PRIVATE 
  util_log
)
