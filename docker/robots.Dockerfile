FROM ubuntu:24.04
ENV DEBIAN_FRONTEND=noninteractive

RUN apt-get update \
    && apt-get install -y --no-install-recommends \
       build-essential \
       cmake \
       pkg-config \
    libflatbuffers-dev \
    flatbuffers-compiler \
       libzmq3-dev \
       libgrpc++-dev \
       libprotobuf-dev \
       protobuf-compiler \
       protobuf-compiler-grpc \
       ca-certificates \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /src/robots
COPY robots /src/robots
COPY inventory/proto/inventory.proto /src/robots/proto/inventory.proto
RUN flatc --cpp -o /src/robots/fbs /src/robots/fbs/order.fbs
RUN protoc -I. --cpp_out=. --grpc_out=. --plugin=protoc-gen-grpc=/usr/bin/grpc_cpp_plugin proto/inventory.proto
RUN cmake -S . -B build && cmake --build build -j

WORKDIR /src/robots/build
ENTRYPOINT ["./robot_worker"]
