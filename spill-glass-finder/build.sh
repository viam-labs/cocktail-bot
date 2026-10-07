#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")"

tar --exclude __pycache__ -czf module.tar.gz meta.json README.md requirements.txt setup.sh run.sh src
