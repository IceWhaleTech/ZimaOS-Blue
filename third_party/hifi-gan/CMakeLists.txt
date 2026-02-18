cmake_minimum_required(VERSION 3.10)
project(hifigan)

set(CMAKE_CXX_STANDARD 17)
set(CMAKE_CXX_STANDARD_REQUIRED ON)

# Build static library
add_library(hifigan STATIC
    src/hifigan.cpp
)

target_include_directories(hifigan PUBLIC
    ${CMAKE_CURRENT_SOURCE_DIR}/src
)

# Set output directory
set_target_properties(hifigan PROPERTIES
    ARCHIVE_OUTPUT_DIRECTORY "${CMAKE_CURRENT_SOURCE_DIR}/build"
)
