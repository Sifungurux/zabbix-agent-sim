# ── Stage 1: Build the Go metrics server ──────────────────────────────────────
FROM golang:1.22-alpine AS builder
WORKDIR /build
COPY go.mod .
COPY metrics.go server.go server_test.go ./
# Tests run here on Linux — all tests execute against real /proc
RUN go test ./... && \
    CGO_ENABLED=0 go build -ldflags="-s -w" -o metrics-server .

# ── Stage 2: Final image ───────────────────────────────────────────────────────
FROM alpine:3.19
# zabbix-agent2 is in the community repo; --repository scopes it to this invocation only
RUN apk add --no-cache zabbix-agent2 \
    --repository=https://dl-cdn.alpinelinux.org/alpine/v3.19/community
COPY --from=builder /build/metrics-server /usr/local/bin/metrics-server
COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh /usr/local/bin/metrics-server
EXPOSE 8080
ENTRYPOINT ["/entrypoint.sh"]
