#!/bin/bash
SERVICES="portal authentik nextcloud odoo stalwart bulwark"
for svc in $SERVICES; do
  if ! docker ps --format '{{.Names}}' | grep -q "$svc"; then
    echo "⚠️ Service $svc tidak jalan!"
  fi
done
