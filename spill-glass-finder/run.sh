#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")"

if [ ! -f .venv/.setup-complete ]; then
    ./setup.sh >&2
fi
exec .venv/bin/python src/main.py "$@"
