app_name := "s99deploy"

# List available commands.
default:
    @just --list

# Build the CLI and site binaries.
build:
    mkdir -p bin
    go build -trimpath -o bin/{{app_name}} .
    go build -trimpath -o bin/{{app_name}}-site ./site

# Run the hosting site locally.
site: build
    CONFIG_PATH=config.json ./bin/{{app_name}}-site

# Install the binary system-wide (run this on the VPS).
install: build
    sudo install -m 0755 bin/{{app_name}} /usr/local/bin/{{app_name}}

# Run Go tests.
test:
    go test ./...

# Run Go static analysis.
vet:
    go vet ./...

# Format Go source files.
fmt:
    gofmt -s -w main.go manifest deploy site

# Regenerate Go manifest types from the CUE schema.
gen-types:
    bash scripts/gen-types.sh

# Run all validation used before committing.
check: vet test build

# Remove generated output.
clean:
    rm -rf bin
