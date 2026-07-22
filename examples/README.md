# Examples

A manifest is one `deploy.json` in the app repo root. Two shapes cover most
apps:

- [`minimal/`](minimal/deploy.json) - a Go service: build one binary, run it,
  health-check one port.
- [`webapp/`](webapp/deploy.json) - a Go binary serving a prebuilt frontend:
  pnpm build first, extra env, a required secret from `.env`, a slower health
  check.

## Full lifecycle

One-time setup on the VPS (clones the repo, creates the `myapp` user and
`/opt/myapp`, installs the systemd unit):

```sh
sudo s99deploy install git@github.com:smegg99/MyApp.git
sudo nano /opt/myapp/.env    # fill in secrets
sudo s99deploy up myapp
```

Every deploy after that is one command:

```sh
sudo s99deploy up myapp
```

Check on it:

```sh
systemctl status myapp
journalctl -u myapp -f
```

Remove it (keeps `/opt/myapp` and the user unless you add `--purge`):

```sh
sudo s99deploy uninstall myapp
```
