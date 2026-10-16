#!/usr/bin/env bash
set -euo pipefail
YANDU_ROOT="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
exec bash "$YANDU_ROOT/scripts/upgrade-cloud.sh" "$@"
