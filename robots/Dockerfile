# STAGE 1: Build the binary
FROM ubuntu:24.04 AS builder

ENV DEBIAN_FRONTEND=noninteractive

# Added libflatbuffers-dev to provide the missing flatbuffers/flatbuffers.h
RUN apt-get update && apt-get install -y \
    build-essential \
    cmake \
    pkg-config \
    libzmq3-dev \
    libgrpc++-dev \
    protobuf-compiler-grpc \
    libprotobuf-dev \
    flatbuffers-compiler \
    libflatbuffers-dev \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app
COPY . .

# A. Generate Flatbuffers
RUN flatc --cpp -o fbs/ fbs/order.fbs

# B. Generate Protobuf/gRPC
RUN protoc -I proto --cpp_out=proto --grpc_out=proto \
    --plugin=protoc-gen-grpc=`which grpc_cpp_plugin` \
    proto/inventory.proto

# C. Build the C++ binary
RUN mkdir build && cd build && \
    cmake .. && \
    make -j$(nproc)

# STAGE 2: Runtime
FROM ubuntu:24.04
RUN apt-get update && apt-get install -y \
    libzmq5 \
    libgrpc++1.51 \
    libprotobuf32 \
    && rm -rf /var/lib/apt/lists/*

COPY --from=builder /app/build/robot_worker /usr/local/bin/robot_worker

ENTRYPOINT ["robot_worker"]