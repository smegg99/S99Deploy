// internal/site/install.go

package site

// usageText is what a person reading / gets.
const usageText = `depl %[2]s -- deploy manifest-carrying apps to /opt under systemd

Install on a Linux x86-64 VPS:

  curl -fsSL "%[1]s/install.sh" | sudo sh

Or by hand, checking the download yourself:

  curl -fsSL "%[1]s/depl" -o depl
  curl -fsSL "%[1]s/depl.sha256" | sha256sum -c -
  sudo install -m 0755 depl /usr/local/bin/depl

Docs: https://github.com/smegg99/S99Deploy
`

// installScript verifies what it downloaded before it installs it.
const installScript = `#!/bin/sh
set -eu

main() {
    url="%[1]s/depl"
    want="%[3]s"
    dest="/usr/local/bin/depl"

    tmp="$(mktemp)"
    trap 'rm -f "$tmp"' EXIT

    curl -fsSL "$url" -o "$tmp"

    got="$(sha256sum "$tmp" | cut -d' ' -f1)"
    if [ "$got" != "$want" ]; then
        echo "depl: checksum mismatch for $url" >&2
        echo "  expected $want" >&2
        echo "  got      $got" >&2
        echo "  nothing was installed; retry, or fetch $url and %[1]s/depl.sha256 by hand" >&2
        exit 1
    fi

    install -m 0755 "$tmp" "$dest"
    echo "installed $dest (depl %[2]s)"
}

main "$@"
`
