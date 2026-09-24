# Examples

A manifest is one `deploy.json` in the app repo root. Two shapes cover most
apps:

- [`minimal/`](minimal/deploy.json) - a Go service: build one binary, run it,
  health-check one port.
- [`webapp/`](webapp/deploy.json) - a Go binary serving a prebuilt frontend:
  pnpm build first, extra env, a required secret from `.env`, a slower health
  check.

## Full lifecycle

One-time setup on the VPS (clones the repo, creates the `myapp` account and
`/opt/myapp`, installs the systemd unit):

```sh
sudo --preserve-env=SSH_AUTH_SOCK depl install git@github.com:smegg99/MyApp.git
sudo nano /opt/myapp/.env    # fill in secrets
sudo depl up myapp
```

Every deploy after that is one command:

```sh
sudo depl up myapp
```

Check on it:

```sh
systemctl status myapp
journalctl -u myapp -f
```

Remove it. The unit goes; the checkout, `.env` and the account stay:

```sh
sudo depl uninstall myapp
```

It asks you to type `myapp` before it does anything. `--yes` skips that, for a
script. `--purge` also deletes `/opt/myapp` and the `myapp` account, after
checking that `/opt/myapp` really is that account's home:

```sh
sudo depl uninstall --purge --yes myapp
```
