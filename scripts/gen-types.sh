#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT/manifest"

if ! command -v cue >/dev/null 2>&1; then
    echo "cue not found in PATH. Install: https://cuelang.org/docs/install/" >&2
    exit 1
fi

rm -f cue_types_manifest_gen.go
cue exp gengotypes .
gofmt -s -w cue_types_manifest_gen.go
echo "regenerated manifest/cue_types_manifest_gen.go"
