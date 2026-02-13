# STAGE 1: Build
FROM ubuntu:24.04 AS builder
ENV DEBIAN_FRONTEND=noninteractive

RUN apt-get update && apt-get install -y \
    build-essential cmake pkg-config libzmq3-dev \
    flatbuffers-compiler libflatbuffers-dev && rm -rf /var/lib/apt/lists/*

WORKDIR /app
# Copy from root so we have access to the whole analytics folder context
COPY . .

WORKDIR /app/analytics

# Generate Flatbuffers headers [cite: 107, 121]
RUN flatc --cpp -o fbs/ fbs/analytics.fbs

# Build binary
RUN mkdir build && cd build && cmake .. && make -j$(nproc)

# STAGE 2: Runtime
FROM ubuntu:24.04
RUN apt-get update && apt-get install -y libzmq5 && rm -rf /var/lib/apt/lists/*

COPY --from=builder /app/analytics/build/analytics_service /usr/local/bin/analytics_service

# Persistence for your CSV metrics [cite: 145]
VOLUME /data
WORKDIR /data

ENTRYPOINT ["analytics_service"]