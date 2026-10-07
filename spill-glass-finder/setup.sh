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
.venv/bin/python -m pip install --index-url https://download.pytorch.org/whl/cpu "torch==2.2.2" "torchvision==0.17.2"
.venv/bin/python -m pip install -r requirements.txt
# Not on PyPI. --no-deps skips its training-only pins (fiftyone, albumentations, pre-commit, pytest);
# requirements.txt carries the ones its load path imports.
.venv/bin/python -m pip install --no-deps \
    "keypoint-detection @ git+https://github.com/tlpss/keypoint-detection@778f087e8060300aeda978a8a7e943822ea8b5ba"
touch .venv/.setup-complete
