# S99Deploy

One deploy CLI for every same-shape app on a Linux VPS: a Go binary (optionally
serving a prebuilt frontend) built on the server from a git checkout, run as a
dedicated system account under systemd, listening on a loopback port behind a
reverse proxy.

Instead of each app repo carrying its own `deploy.sh`, `install-systemd.sh` and
unit template, the app carries one `deploy.json` and the box carries one
`s99deploy` binary. The unit is generated from a single template whose
`ExecStart` is `s99deploy run`, so every app's unit is the same file with three
values filled in.

Linux and systemd only. The site at `/s99deploy` serves a linux/amd64 binary.

Built with [cobra](https://github.com/spf13/cobra),
[s99config](https://github.com/smegg99/s99config) (CUE-schema validation),
[s99logger](https://github.com/smegg99/s99logger) and
[s99term](https://github.com/smegg99/s99term).

## Prerequisites

On the VPS, to run it:

- systemd, `git`, `bash`, `useradd` and `userdel` (shadow-utils)
- whatever the apps build with (Go toolchain, Node and pnpm, ...). `s99deploy`
  prepends `/usr/local/go/bin` to `PATH` for builds when that directory exists

To build it from source: Go 1.27 and [just](https://github.com/casey/just).

## Install

From source, on the VPS:

```sh
git clone https://github.com/smegg99/S99Deploy.git
cd S99Deploy
just install
```

Or from a running [hosting site](#hosting-the-binary), which verifies the
download against the checksum the site publishes:

```sh
curl -fsSL https://<your-host>/install.sh | sudo sh
```

The script downloads to a temporary file, compares its sha256 with the digest
the site read from the same file it serves, and installs nothing on a mismatch.
The script and the binary come from one origin, so this catches a truncated or
corrupted download, not a compromised host: TLS is the trust boundary. The
digest is also served on its own at `/s99deploy.sha256`, in `sha256sum -c`
format.

## The manifest

Each app repo carries a `deploy.json` in its root, validated against
[`internal/manifest/schema.cue`](internal/manifest/schema.cue) on every load:

```json
{
  "name": "smeggtunersite",
  "build": [
    "cd web && pnpm install --frozen-lockfile",
    "cd web && pnpm build",
    "go build -trimpath -o bin/smeggtunersite ."
  ],
  "run": ["bin/smeggtunersite"],
  "env": { "CONFIG_PATH": "config.json" },
  "require_env": ["SMEGGTUNER_SITE_URL"],
  "check": { "port": 9245 }
}
```

| Field | Default | Description |
| --- | --- | --- |
| `name` | required | Service name: the systemd unit, the system account, and `/opt/<name>`. Lowercase, digits, hyphens. |
| `build` | `[]` | Shell commands run in order as the service account in the app dir, each through `bash -lc`. They must be non-interactive. |
| `run` | required | Argv exec'd by systemd. `argv[0]` with a path separator is resolved relative to the app dir; a bare name through `PATH`. |
| `env` | `{}` | Extra environment for build and run. |
| `require_env` | `[]` | Vars that must be non-empty in the build environment, usually from `.env`. |
| `check.port` | required | Loopback port for the post-deploy health check. |
| `check.path` | `/` | Health check path; must answer 200. |
| `check.timeout_seconds` | `10` | How long the health check waits before the deploy fails. |

## Commands

```sh
sudo s99deploy install <git-url>              # one-time app setup
sudo s99deploy up <name> [--timeout 60s]      # deploy
sudo s99deploy uninstall [--purge] [--yes] <name>
s99deploy run <root>                          # systemd ExecStart, not for people
```

`install` clones the repo, reads the manifest, creates the `<name>` system
account (no password, no sudo), lays out `/opt/<name>` with the checkout at
`/opt/<name>/app` and the account's home at `/opt/<name>/home` at mode 0700,
seeds `/opt/<name>/.env` from `app/.env.example` at mode 0600, and installs and
enables the unit. It is safe to rerun. It refuses to adopt an existing account
whose home is not `/opt/<name>`, because adopting one is how a later `--purge`
deletes the wrong directory; it refuses a name that
already has a unit somewhere on systemd's search path, and a unit file it did
not write itself. `/opt/<name>` stays root-owned, and `.env` root-owned at
mode 0600; a rerun takes both back if an older install left them to the account.
The account owns `app/` and `home/` inside it. `home/` is the `HOME` every build
command and the service itself run with: `go build` writes its build cache
there, `pnpm` and `npm` their `~/.cache` and `~/.local`, and a `HOME` the
account cannot write fails all of them. passwd still names `/opt/<name>` as the
account's home, because that is the directory `--purge` checks before it
deletes.

`up` pulls `--ff-only`, rereads the manifest, loads `/opt/<name>/.env`, checks
`require_env`, runs the build commands as the service account, restarts the
unit, and waits for the health check. `--timeout` overrides the manifest's
`check.timeout_seconds` for that one run. The wait folds the unit's own state
into every attempt, so a service that crash-loops fails in about five seconds
instead of at the timeout. A cancelled build leaves the checkout at the new
commit with a partial `bin/`; the service is not restarted, so it cannot take
the app down, and the recovery is to run `up` again. There is no rollback: to go
back, run `git -C /opt/<name>/app reset --hard <commit>` and then `up`.

`run` loads the manifest, applies its `env` and execs the app, so systemd
supervises the real process. Secrets reach it through the unit's
`EnvironmentFile=/opt/<name>/.env`.

`uninstall` asks you to type the app's name, then disables and removes the unit.
`--yes` skips the prompt. The checkout, `.env` and account stay unless
`--purge`, which removes `/opt/<name>` by path after checking that it is the
account's home, and then runs `userdel` without `-r`. An account somebody has
already deleted is not an error: the tree still goes.

Exit codes: 0 for success, 1 for work that ran and failed, 2 for a usage
mistake, which also prints the usage block. Both binaries.

`--lang en|pl` picks the language, ahead of `S99DEPLOY_LANG`, `LC_ALL`,
`LC_MESSAGES` and `LANG`. `--color auto|always|never` decides colour; `auto`
honours `NO_COLOR`, `CLICOLOR_FORCE` and a stream that is not a terminal.
`--verbose` logs every step instead of the ones that matter. Every human line
goes to stderr; stdout carries nothing but machine-readable output. Child
processes keep their own descriptors, so build output is untouched.

Ready-made manifests and a full lifecycle walkthrough live in
[`examples/`](examples/).

## Private repositories

The first clone runs as root, and `sudo` drops `SSH_AUTH_SOCK` under its default
`env_reset`, so agent forwarding alone is not enough:

```sh
sudo --preserve-env=SSH_AUTH_SOCK s99deploy install git@github.com:smegg99/MyApp.git
```

`install` checks for the socket before it clones an SSH URL and tells you this
if it is missing, rather than hanging on a prompt. Later pulls run as the
service account, which needs its own read-only deploy key in
`/opt/<name>/.ssh`. OpenSSH expands `~` from passwd, not from `HOME`, so the key
belongs there and not under `/opt/<name>/home`; git's own `.gitconfig` is read
from `HOME`. `/opt/<name>` and its `.env` are root-owned so the service cannot
read or replace the secrets, so create that `.ssh` yourself and give it to the
account (`install -d -o <name> -g <name> -m 0700 /opt/<name>/.ssh`), and note
the service writes only under `/opt/<name>/app` and `/opt/<name>/home`. Both
the clone and the pulls run with `GIT_TERMINAL_PROMPT=0` and
`ssh -o BatchMode=yes`, so a missing key fails in a second instead of waiting
for input nothing will type.

## What the unit forbids

[`internal/deploy/unit.service.tmpl`](internal/deploy/unit.service.tmpl) is the
one unit every app gets. It runs as `User=<name>` with `NoNewPrivileges`,
private `/tmp` and `/dev`, a read-only `/usr` and `/etc` (`ProtectSystem=strict`)
with `/opt/<name>/app` and `/opt/<name>/home` as the only writable paths, no
access to other users' home
directories, no kernel tunables, no kernel modules, no control-group writes, no
namespaces, no `personality(2)` changes, and no writable-executable memory.
`MemoryDenyWriteExecute=true` means an app that runs a JIT under systemd will
not start; every app deployed this way today is a Go binary, and Node runs at
build time only, outside the unit.

`RestrictAddressFamilies=AF_UNIX AF_INET AF_INET6`. AF_UNIX is there because the
resolver, journald's `/dev/log`, D-Bus and any unix-socket daemon need it;
AF_PACKET and AF_NETLINK, the families the option exists to block, stay blocked.
Units written before this change carry the older line and keep it until the app
is installed again.

## Hosting the binary

`internal/site` is a small Gin server that makes the CLI curl-able:

| Endpoint | Serves |
| --- | --- |
| `/` | Plain-text usage with copy-pasteable install commands. |
| `/install.sh` | A script that verifies the checksum before installing. |
| `/s99deploy` | The linux/amd64 binary, built at deploy time from this repo. |
| `/s99deploy.sha256` | The digest of that binary, in `sha256sum -c` format. |

The printed URLs follow the request host, so the site works on any domain behind
a reverse proxy:

```caddy
get.example.com {
    reverse_proxy 127.0.0.1:9250
}
```

A request whose `Host` is not a plain name, address or bracketed IPv6 literal
gets a 400 instead, because those two endpoints put the host in a command that
can reach a root shell through `curl ... | sudo sh`.

Config lives in [`config.json`](config.json) (`gin_mode`, `listen_addr`,
`bin_path`, `trusted_proxies`), validated by
[`internal/site/schema.cue`](internal/site/schema.cue). `trusted_proxies` is the
set of peers, as bare addresses or CIDR blocks, whose `X-Forwarded-Proto` is
believed when the site builds those URLs, and whose `X-Forwarded-For` gin
believes for `ClientIP`. An empty list trusts nobody, and a request from any
other peer gets `http://` unless it arrived over TLS directly.

The repo carries its own [`deploy.json`](deploy.json), so the site is deployed
with the tool it hosts:

```sh
sudo --preserve-env=SSH_AUTH_SOCK s99deploy install git@github.com:smegg99/S99Deploy.git
sudo s99deploy up s99deploy-site
```

Run it locally with `just site`.

## Development

```sh
just check        # check-fmt, vet, test, build, check-cue, check-manifests, check-locales, check-version
just fmt          # rewrite; check-fmt only reports
just gen-types    # regenerate the CUE-derived Go types with the pinned cue
just locales      # rebuild the locale bundle with the pinned loc
```

`cue` and `loc` are pinned by go.mod tool directives, so `just` never reads
either from `PATH`. Generated output is committed: the CUE types beside each
schema, and the locale bundle with loc's manifest in `internal/messages`.

## License

See [LICENSE](LICENSE).
