#!/bin/bash
# DEPRECATED shim — canonical version lives at infra/scripts/phase1/05-node-exporter.sh
# Kept temporarily so existing automation doesn't break; will be removed when the
# duplicated script trees are consolidated (Phase 1).
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
exec bash "${SCRIPT_DIR}/../../scripts/phase1/05-node-exporter.sh" "$@"
