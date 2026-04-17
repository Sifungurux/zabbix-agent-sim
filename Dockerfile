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
# jq is in main; zabbix-agent2 is in community
RUN apk add --no-cache jq && \
    apk add --no-cache zabbix-agent2 \
    --repository=https://dl-cdn.alpinelinux.org/alpine/v3.19/community
COPY --from=builder /build/metrics-server /usr/local/bin/metrics-server
COPY entrypoint.sh deregister.sh /usr/local/bin/
RUN chmod +x /usr/local/bin/entrypoint.sh /usr/local/bin/deregister.sh /usr/local/bin/metrics-server
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
