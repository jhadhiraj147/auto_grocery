cmake_minimum_required(VERSION 3.10)
project(RobotService)

set(CMAKE_CXX_STANDARD 17)
set(CMAKE_POSITION_INDEPENDENT_CODE ON)

# Tell CMake to look in your local directory for FindZeroMQ.cmake
set(CMAKE_MODULE_PATH ${CMAKE_MODULE_PATH} "${CMAKE_CURRENT_SOURCE_DIR}/include/cppzmq/libzmq-pkg-config")

# 1. Find Dependencies
find_package(Threads REQUIRED)
find_package(ZeroMQ REQUIRED) # Changed from ZMQ to ZeroMQ to match typical Find modules
find_package(gRPC REQUIRED)
find_package(Protobuf REQUIRED)

# 2. Define Include Directories
include_directories(
    ${CMAKE_CURRENT_SOURCE_DIR}
    ${CMAKE_CURRENT_SOURCE_DIR}/proto
    ${CMAKE_CURRENT_SOURCE_DIR}/fbs
    ${CMAKE_CURRENT_SOURCE_DIR}/include/cppzmq
)

# 3. Source Files
set(SOURCES
    internal/bot/main.cpp
    proto/inventory.pb.cc
    proto/inventory.grpc.pb.cc
)

# 4. Create Executable
add_executable(robot_worker ${SOURCES})

# 5. Link Libraries
target_link_libraries(robot_worker
    ${ZeroMQ_LIBRARIES}
    gRPC::grpc++
    protobuf::libprotobuf
    Threads::Threads
)