#!/usr/bin/env bash
# Downloads SPILL's wild_glasses.ckpt (MIT, https://github.com/Louadria/SPILL) into weights/ and checks its sha256.
set -euo pipefail
cd "$(dirname "$0")/.."

URL="https://media.githubusercontent.com/media/Louadria/SPILL/main/checkpoints/wild_glasses.ckpt"
SHA256="ab04039ccd429d3063498749723958438124f484d2c0a63e5727b9e70a576b11"
DEST="weights/wild_glasses.ckpt"

mkdir -p weights
curl -fsSL -o "$DEST.part" "$URL"
if command -v sha256sum >/dev/null; then
    actual="$(sha256sum "$DEST.part" | cut -d' ' -f1)"
else
    actual="$(shasum -a 256 "$DEST.part" | cut -d' ' -f1)"
fi
if [ "$actual" != "$SHA256" ]; then
    echo "sha256 mismatch for $URL: got $actual, want $SHA256" >&2
    exit 1
fi
mv "$DEST.part" "$DEST"
echo "wrote $DEST"
