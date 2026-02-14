FROM golang:1.25-alpine AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/pricing ./pricing/cmd/pricing

FROM alpine:3.20
RUN adduser -D -u 10001 appuser
WORKDIR /app
COPY --from=build /bin/pricing /app/pricing
USER appuser
EXPOSE 50052
ENTRYPOINT ["/app/pricing"]
