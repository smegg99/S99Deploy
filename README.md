# S99Deploy

One deploy CLI for every same-shape app on a VPS: a Go binary (optionally
serving a prebuilt web frontend) built on the server from a git checkout, run
as a dedicated system user under systemd, listening on a loopback port behind
a reverse proxy.

Instead of each app repo carrying its own copy of `deploy.sh`,
`install-systemd.sh`, and a unit template, the app carries one `deploy.json`
manifest and the box carries one `s99deploy` binary. The systemd unit is
generated from a single hardened template whose `ExecStart` is
`s99deploy run`, so units are identical boilerplate across apps.

Built with Go, [cobra](https://github.com/spf13/cobra),
[s99config](https://github.com/smegg99/s99config) (CUE-schema manifest
validation), and [s99logger](https://github.com/smegg99/s99logger).

## Prerequisites

On the VPS:

- systemd, `git`, `runuser` (util-linux), `useradd`
- whatever the apps build with (Go toolchain, Node + pnpm, ...); `s99deploy`
  prepends `/usr/local/go/bin` to `PATH` for builds when it exists

## Setup

Build and install the CLI system-wide on the VPS:

```sh
git clone https://github.com/smegg99/S99Deploy.git
cd S99Deploy
just install   # go build + sudo install to /usr/local/bin/s99deploy
```

## The manifest

Each app repo carries a `deploy.json` in its root, validated against a CUE
schema on every load:

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

| Field                   | Default  | Description                                                                                                              |
| ----------------------- | -------- | ------------------------------------------------------------------------------------------------------------------------ |
| `name`                  | required | Service name: the systemd unit, the system user, and `/opt/<name>`. Lowercase, digits, hyphens.                          |
| `build`                 | `[]`     | Shell commands run in order as the service user in the app dir.                                                          |
| `run`                   | required | Argv exec'd by systemd. `argv[0]` with a path separator is resolved relative to the app dir; a bare name through `PATH`. |
| `env`                   | `{}`     | Extra environment for build and run (e.g. `CONFIG_PATH`).                                                                |
| `require_env`           | `[]`     | Vars that must be non-empty in the build environment (usually from `.env`).                                              |
| `check.port`            | required | Loopback port for the post-deploy health check.                                                                          |
| `check.path`            | `/`      | Health check path; must return 200.                                                                                      |
| `check.timeout_seconds` | `10`     | How long the health check polls before the deploy fails.                                                                 |

## Commands

```sh
sudo s99deploy install <git-url>       # one-time app setup
sudo s99deploy up <name>               # deploy
sudo s99deploy uninstall [--purge] <name>
s99deploy run <root>                   # systemd ExecStart, not for manual use
```

`install` clones the repo, reads the manifest, creates the `<name>` system
user (no sudo, no password), lays out `/opt/<name>` with the checkout at
`/opt/<name>/app`, seeds `/opt/<name>/.env` from `.env.example` (0600), and
installs and enables the systemd unit. Safe to rerun.

`up` pulls `--ff-only`, re-reads the manifest, loads `/opt/<name>/.env`,
checks `require_env`, runs the build commands as the service user, restarts
the service, and polls the health check. Any failure aborts with a pointer to
`journalctl -u <name>`.

`run` loads the manifest, applies its `env`, and execs the app binary, so
systemd supervises the real process. Secrets reach it through the unit's
`EnvironmentFile=/opt/<name>/.env`.

`uninstall` asks you to type the name, then disables and removes the unit.
The checkout, `.env`, and user stay unless `--purge` (which runs
`userdel -r`).

Ready-made manifests and a full lifecycle walkthrough live in
[`examples/`](examples/).

## Deploying an app

```sh
sudo s99deploy install git@github.com:smegg99/SmeggTunerSite.git
sudo nano /opt/smeggtunersite/.env      # fill in secrets
sudo s99deploy up smeggtunersite
```

For private repos the initial clone runs as root, so SSH agent forwarding is
enough; later pulls run as the service user, which needs its own read-only
deploy key in `/opt/<name>/.ssh`.

Status and logs:

```sh
systemctl status smeggtunersite
journalctl -u smeggtunersite -f
```

## Development

```sh
just check       # vet + test + build
just gen-types   # regenerate Go manifest types from the CUE schema (needs cue)
```

## License

See [LICENSE](LICENSE).
