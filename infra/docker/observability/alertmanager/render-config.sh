#!/bin/sh
# Renders alertmanager.yml from the template by substituting secret env vars,
# then hands off to the real alertmanager binary. Runs inside the official
# prom/alertmanager image (busybox sh).
set -eu

TMPL=/etc/alertmanager/alertmanager.yml.tmpl
OUT=/etc/alertmanager/alertmanager.yml

sed \
    -e "s|\${SMTP_PASSWORD}|${SMTP_PASSWORD}|g" \
    -e "s|\${DISCORD_WEBHOOK_URL}|${DISCORD_WEBHOOK_URL}|g" \
    "$TMPL" > "$OUT"

chmod 600 "$OUT"
exec /bin/alertmanager --config.file="$OUT" --storage.path=/alertmanager
