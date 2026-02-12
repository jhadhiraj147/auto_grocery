cmake_minimum_required(VERSION 3.10)
project(AnalyticsService)

set(CMAKE_CXX_STANDARD 17)

# Point CMake to your local FindZeroMQ.cmake module [cite: 110, 116]
set(CMAKE_MODULE_PATH ${CMAKE_MODULE_PATH} "${CMAKE_CURRENT_SOURCE_DIR}/include/cppzmq/libzmq-pkg-config")

find_package(Threads REQUIRED)
find_package(ZeroMQ REQUIRED) # This will now use your local FindZeroMQ.cmake

include_directories(
    ${CMAKE_CURRENT_SOURCE_DIR}/include
    ${CMAKE_CURRENT_SOURCE_DIR}/include/cppzmq
    ${CMAKE_CURRENT_SOURCE_DIR}/fbs
)

# Use the binary name specified in your structure [cite: 107]
add_executable(analytics_service cmd/analytics/main.cpp)

target_link_libraries(analytics_service
    ${ZeroMQ_LIBRARIES}
    Threads::Threads
)