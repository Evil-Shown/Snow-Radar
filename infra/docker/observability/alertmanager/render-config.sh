#!/bin/sh
# Renders alertmanager.yml from the template by substituting secret env vars,
# then hands off to the real alertmanager binary. Runs inside the official
# prom/alertmanager image (busybox sh).
#
# AUDIT FINDING #11: naive `sed s|...|...|` breaks (or worse, mis-substitutes)
# when a secret contains the delimiter. Values are passed via environment and
# referenced as \&-safe replacements using awk instead of sed.
set -eu

TMPL=/etc/alertmanager/alertmanager.yml.tmpl
OUT=/etc/alertmanager/alertmanager.yml

SMTP_PASSWORD="${SMTP_PASSWORD}" DISCORD_WEBHOOK_URL="${DISCORD_WEBHOOK_URL}" awk '
{
    gsub(/\$\{SMTP_PASSWORD\}/, ENVIRON["SMTP_PASSWORD"])
    gsub(/\$\{DISCORD_WEBHOOK_URL\}/, ENVIRON["DISCORD_WEBHOOK_URL"])
    print
}' "$TMPL" > "$OUT"

chmod 600 "$OUT"
exec /bin/alertmanager --config.file="$OUT" --storage.path=/alertmanager
