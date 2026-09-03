# justfile
app_name := "s99deploy"

# List available commands.
default:
    @just --list

# Every check that gates a commit.
check: check-fmt vet test build check-cue check-manifests

# Fail when any Go file is not gofmt clean. Never rewrites.
check-fmt:
    #!/usr/bin/env bash
    set -euo pipefail
    unformatted="$(gofmt -s -l .)"
    if [ -n "$unformatted" ]; then
        echo "gofmt -s would rewrite:" >&2
        echo "$unformatted" >&2
        exit 1
    fi

# Rewrite every Go file with gofmt. Not part of check.
fmt:
    gofmt -s -w .

# Run go vet.
vet:
    go vet ./...

# Run all tests.
test:
    go test ./...

# Build both binaries into bin/.
build:
    mkdir -p bin
    go build -trimpath -o bin/{{app_name}} ./cmd/s99deploy
    go build -trimpath -o bin/{{app_name}}-site ./cmd/s99deploy-site

# Run the hosting site locally.
site: build
    CONFIG_PATH=config.json ./bin/{{app_name}}-site

# Install the CLI system-wide (run this on the VPS).
install: build
    sudo install -m 0755 bin/{{app_name}} /usr/local/bin/{{app_name}}

# Regenerate the Go types from the CUE schemas with the pinned cue.
gen-types:
    #!/usr/bin/env bash
    set -euo pipefail
    for dir in internal/manifest internal/site; do
        cd "{{justfile_directory()}}/$dir"
        go tool cue exp gengotypes .
        gofmt -s -w cue_types_*_gen.go
    done

# Fail when the committed CUE-generated Go drifts from the schemas.
check-cue:
    #!/usr/bin/env bash
    # The deleted script removed the generated files before regenerating, so a
    # failed run left the tree short a file. This never touches the tree.
    set -euo pipefail
    root="{{justfile_directory()}}"
    tmp="$(mktemp -d)"
    trap 'rm -rf "$tmp"' EXIT
    go build -o "$tmp/cue" cuelang.org/go/cmd/cue
    for dir in internal/manifest internal/site; do
        name="$(basename "$dir")"
        mkdir -p "$tmp/$name"
        cp "$root/$dir/schema.cue" "$tmp/$name/"
        (cd "$tmp/$name" && "$tmp/cue" exp gengotypes . >/dev/null && gofmt -s -w cue_types_*_gen.go)
        for generated in "$tmp/$name"/cue_types_*_gen.go; do
            diff -u "$root/$dir/$(basename "$generated")" "$generated"
        done
    done

# Validate every manifest this repo ships against the schema the binary embeds.
check-manifests:
    #!/usr/bin/env bash
    # Globbed, so a new example cannot be added without being validated.
    set -euo pipefail
    root="{{justfile_directory()}}"
    shopt -s nullglob
    for file in "$root/deploy.json" "$root"/examples/*/deploy.json; do
        go tool cue vet -d '#Manifest' "$root/internal/manifest/schema.cue" "$file"
        echo "ok $(realpath --relative-to="$root" "$file")"
    done

# Remove generated output.
clean:
    rm -rf bin
