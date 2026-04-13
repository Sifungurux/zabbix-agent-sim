#!/bin/sh
set -e

# Generate agent config at runtime — $HOSTNAME is injected by Kubernetes
# as the pod name via the HOSTNAME env var (fieldRef: metadata.name).
cat > /tmp/zabbix_agent2.conf << EOF
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
