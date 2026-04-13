# ── Stage 1: Build the Go metrics server ──────────────────────────────────────
FROM golang:1.22-alpine AS builder
WORKDIR /build
COPY go.mod .
COPY *.go ./
# Tests run here on Linux — all tests execute against real /proc
RUN go test ./... && \
    CGO_ENABLED=0 go build -ldflags="-s -w" -o metrics-server .

# ── Stage 2: Final image ───────────────────────────────────────────────────────
FROM alpine:3.19
RUN echo "https://dl-cdn.alpinelinux.org/alpine/v3.19/community" >> /etc/apk/repositories && \
    apk add --no-cache zabbix-agent2
COPY --from=builder /build/metrics-server /usr/local/bin/metrics-server
COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh /usr/local/bin/metrics-server
EXPOSE 8080
ENTRYPOINT ["/entrypoint.sh"]
