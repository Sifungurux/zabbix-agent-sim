#!/bin/sh
# Called by the preStop lifecycle hook — deletes this pod's Zabbix host before shutdown.
# Requires ZABBIX_USER and ZABBIX_PASSWORD env vars (from the zabbix-api-credentials Secret).

ZABBIX_URL="http://zabbix-zabbix-web.zabbix.svc/api_jsonrpc.php"

# Authenticate
TOKEN=$(wget -qO- \
  --header='Content-Type: application/json' \
  --post-data="{\"jsonrpc\":\"2.0\",\"method\":\"user.login\",\"params\":{\"username\":\"${ZABBIX_USER}\",\"password\":\"${ZABBIX_PASSWORD}\"},\"id\":1}" \
  "${ZABBIX_URL}" | jq -r '.result // empty')

if [ -z "$TOKEN" ]; then
  echo "deregister: failed to authenticate with Zabbix API — skipping"
  exit 0
fi

# Look up host by name
HOSTID=$(wget -qO- \
  --header='Content-Type: application/json' \
  --post-data="{\"jsonrpc\":\"2.0\",\"method\":\"host.get\",\"params\":{\"filter\":{\"host\":[\"${HOSTNAME}\"]}},\"auth\":\"${TOKEN}\",\"id\":2}" \
  "${ZABBIX_URL}" | jq -r '.result[0].hostid // empty')

if [ -z "$HOSTID" ]; then
  echo "deregister: host ${HOSTNAME} not found in Zabbix — nothing to delete"
  exit 0
fi

# Delete host
wget -qO- \
  --header='Content-Type: application/json' \
  --post-data="{\"jsonrpc\":\"2.0\",\"method\":\"host.delete\",\"params\":[\"${HOSTID}\"],\"auth\":\"${TOKEN}\",\"id\":3}" \
  "${ZABBIX_URL}" > /dev/null

echo "deregister: deleted host ${HOSTNAME} (hostid=${HOSTID})"
