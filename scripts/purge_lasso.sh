#!/usr/bin/env bash
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BIN="$SCRIPT_DIR/purge_lasso/purge_lasso"

if [ ! -f "$BIN" ]; then
    echo "Building purge_lasso tool..."
    (cd "$SCRIPT_DIR/purge_lasso" && go build -o purge_lasso .)
fi

"$BIN" "$@"
