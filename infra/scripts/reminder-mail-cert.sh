#!/bin/bash
# Reminder renewal mail.pem (cert LE nginx mail vhost) — Sprint 0.10m
# Cert ini TIDAK auto-renew (ditempatkan manual, tidak ada auto-copy dari Stalwart).
# Cron hermes: 0 9 * * * /opt/cloudsuite/infra/scripts/reminder-mail-cert.sh
# Log: /home/hermes/logs/reminder-mail-cert.log

CERT="/opt/cloudsuite/infra/nginx/certs/mail.pem"
DAYS_LEFT=$(( ($(date -d "$(openssl x509 -enddate -noout -in "$CERT" | cut -d= -f2)" +%s) - $(date +%s)) / 86400 ))

echo "$(date '+%Y-%m-%d %H:%M:%S') mail.pem days_left=$DAYS_LEFT"
if [ "$DAYS_LEFT" -lt 30 ]; then
  echo "WARNING: mail.pem expires in $DAYS_LEFT days (Dec 10 2026?) — renew manual + copy ke $CERT + docker exec cloudsuite-nginx nginx -s reload"
  exit 1
fi
exit 0
