#!/bin/bash
THRESHOLD=80
USAGE=$(df / | tail -1 | awk '{print $5}' | sed 's/%//')
if [ $USAGE -gt $THRESHOLD ]; then
  echo "⚠️ Storage VPS $USAGE% — perlu cleanup!"
  # nanti: kirim alert ke Telegram/email
fi
