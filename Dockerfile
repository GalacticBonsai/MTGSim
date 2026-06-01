# syntax=docker/dockerfile:1
FROM golang:latest AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o bin/mtgsim-edh ./cmd/mtgsim-edh

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /app

COPY --from=builder /app/bin/mtgsim-edh /usr/local/bin/mtgsim-edh

# Default data directories
RUN mkdir -p /data /app/decks /app/.cache

VOLUME ["/data", "/app/decks", "/app/.cache"]
EXPOSE 8080

CMD ["mtgsim-edh", "-port=8080"]
