FROM golang:1.25-alpine AS build
WORKDIR /src

RUN apk add --no-cache build-base pkgconf zeromq-dev

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=1 GOOS=linux go build -o /bin/inventory ./inventory/cmd/inventory

FROM alpine:3.20
RUN apk add --no-cache libzmq
RUN adduser -D -u 10001 appuser
WORKDIR /app
COPY --from=build /bin/inventory /app/inventory
USER appuser
EXPOSE 50051 5556
ENTRYPOINT ["/app/inventory"]
