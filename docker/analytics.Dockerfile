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
       ca-certificates \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /src/analytics
COPY analytics /src/analytics
RUN flatc --cpp -o /src/analytics/fbs /src/analytics/fbs/analytics.fbs
RUN cmake -S . -B build && cmake --build build -j

WORKDIR /src/analytics/build
ENTRYPOINT ["./analytics_service"]
