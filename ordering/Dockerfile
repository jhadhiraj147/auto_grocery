FROM golang:1.25-alpine AS build
WORKDIR /src

RUN apk add --no-cache build-base pkgconf zeromq-dev

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=1 GOOS=linux go build -o /bin/ordering ./ordering/cmd/ordering

FROM alpine:3.20
RUN apk add --no-cache libzmq
RUN adduser -D -u 10001 appuser
WORKDIR /app
COPY --from=build /bin/ordering /app/ordering
USER appuser
EXPOSE 5050
ENTRYPOINT ["/app/ordering"]
