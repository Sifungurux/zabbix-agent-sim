#!/bin/sh
set -e

# HOSTNAME must be set — in Kubernetes this comes from fieldRef: metadata.name.
: "${HOSTNAME:?HOSTNAME must be set (set fieldRef: metadata.name in the pod spec)}"

# Generate agent config at runtime — written to /tmp because /etc/zabbix is read-only in the image.
cat > /tmp/zabbix_agent2.conf << EOF
Server=0.0.0.0/0
ServerActive=zabbix-zabbix-server.zabbix.svc.cluster.local
Hostname=${HOSTNAME}
HostMetadata=zabbix-agent-sim
LogType=console
EOF

# Start HTTP metrics server in the background
/usr/local/bin/metrics-server &

# Start Zabbix Agent 2 in the foreground (PID 1 after exec)
# When this process exits the container stops, taking metrics-server with it.
exec zabbix_agent2 -c /tmp/zabbix_agent2.conf
