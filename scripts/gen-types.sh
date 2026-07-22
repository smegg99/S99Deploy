#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

if ! command -v cue >/dev/null 2>&1; then
    echo "cue not found in PATH. Install: https://cuelang.org/docs/install/" >&2
    exit 1
fi

for dir in manifest site; do
    cd "$ROOT/$dir"
    rm -f cue_types_*_gen.go
    cue exp gengotypes .
    gofmt -s -w cue_types_*_gen.go
    echo "regenerated $dir/$(ls cue_types_*_gen.go)"
done
