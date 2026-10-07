#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")"

PYTHON="${PYTHON:-python3}"
if ! "$PYTHON" -c 'import sys; sys.exit(sys.version_info < (3, 11))'; then
    echo "spill-glass-finder needs Python >= 3.11 ($("$PYTHON" --version))" >&2
    exit 1
fi
if [ ! -x .venv/bin/python ]; then
    "$PYTHON" -m venv .venv
fi
.venv/bin/python -m pip install --upgrade pip
# CPU-only torch wheels; PyPI's default x86_64 Linux wheels pull in CUDA.
.venv/bin/python -m pip install --index-url https://download.pytorch.org/whl/cpu "torch>=2.4" "torchvision>=0.19"
.venv/bin/python -m pip install -r requirements.txt
touch .venv/.setup-complete
