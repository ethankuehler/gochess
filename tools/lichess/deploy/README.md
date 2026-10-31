# Deploying gochess to Oracle Cloud Always Free

This turns a fresh Ubuntu VM into a 24/7 Lichess bot: `gochess-bot.service` runs
the [lichess-bot](../README.md) bridge, which spawns `./gochess` per game.

Everything here is deployment tooling. It is not part of the Go build, the
tests, or CI, and nothing in `chess/`, `uci/`, or `cmd/` imports it.

## Before you start

- A Lichess token with the `bot:play` scope. If the bot already runs on your
  laptop, reuse the token from `tools/lichess/.env` — only one process may use
  it at a time, so **stop the laptop copy first** (`Ctrl-C`, and let the
  running games finish).
- An SSH key pair (`~/.ssh/id_ed25519.pub`).
- ~15 minutes.

## 1. Create the instance

In the Oracle Cloud console: **Compute → Instances → Create instance**.

| Field | Value |
|---|---|
| Image | Canonical Ubuntu 24.04 |
| Shape | `VM.Standard.A1.Flex`, **2 OCPU / 12 GB** (the Always Free allowance) |
| Networking | Default VCN, **assign a public IPv4 address** |
| SSH keys | Upload your public key |
| Boot volume | Default (50 GB) |

No ingress rules are needed beyond the default SSH rule: the bot only makes
outbound connections.

`VM.Standard.E2.1.Micro` (1 GB) also works if A1 capacity is unavailable; use
`--low-ram` in step 3. The AMD micro is only 1/8 of an OCPU, so the engine
searches noticeably slower.

## 2. Connect and clone

```sh
ssh ubuntu@<public-ip>
git clone https://github.com/ethankuehler/gochess.git ~/gochess
```

## 3. Install

```sh
~/gochess/tools/lichess/deploy/install.sh
```

It runs as your normal user and uses `sudo` where needed. It:

1. installs `git`, `curl`, `python3-venv`, and upstream Go if the system Go is
   older than `go.mod`'s 1.23.2 (Ubuntu 24.04 ships 1.22);
2. runs `tools/lichess/setup.sh`, which clones lichess-bot, builds the
   virtualenv, installs its Python dependencies, and builds `./gochess`;
3. prompts for the Lichess token and writes `tools/lichess/.env` (mode 600);
4. derives `tools/lichess/config.deploy.yml` from `config.yml`, lowering
   `Hash` to 64 with `--low-ram`;
5. installs `gochess-bot.service` plus a logrotate rule, enables the unit, and
   starts it.

On a 1 GB instance:

```sh
~/gochess/tools/lichess/deploy/install.sh --low-ram
```

Without a terminal (for example when the install is driven over SSH from
another machine) the token prompt cannot work, so pass one of `--token-file
FILE`, `--token T`, or `--no-token`. `--no-token` installs everything, leaves
the unit stopped, and prints the three commands that write `.env` and start the
service.

Re-running the script is the update path: it pulls lichess-bot, rebuilds
gochess, and restarts the service. It never overwrites an existing `.env` or
`config.deploy.yml`.

## 4. Verify

```sh
systemctl status gochess-bot
journalctl -u gochess-bot -n 30 --no-pager
```

Look for `You're now connected to https://lichess.org/`. Then check that
<https://lichess.org/@/gochess6767> shows the bot online, and that a rated game
starts within a couple of minutes (matchmaking waits `challenge_timeout`
minutes between challenges).

## Day 2

| Task | Command |
|---|---|
| Follow logs | `journalctl -u gochess-bot -f` |
| Follow lichess-bot's own log | `tail -f ~/gochess/tools/lichess/lichess-bot.log` |
| Stop (finishes running games) | `sudo systemctl stop gochess-bot` |
| Start | `sudo systemctl start gochess-bot` |
| Update engine + bridge | `cd ~/gochess && git pull && tools/lichess/deploy/install.sh` |
| PGNs | `~/gochess/tools/lichess/games/` |
| Regenerate the per-host config | `rm ~/gochess/tools/lichess/config.deploy.yml` then re-run |

`systemctl stop` sends SIGINT to the bridge only (`KillMode=mixed`) and waits up
to 25 minutes, so a 10+5 game finishes instead of being aborted. `systemctl
restart` uses the same path, so updating mid-game delays the restart.

## Alternative: GitHub Actions

**Off by default — do not enable.** The owner has ruled out Actions hosting (see
`AGENTS.md`). The workflow below stays inert unless the owner explicitly asks
for it.

`lichess-bot.yml` in this directory is an opt-in workflow that runs the bot on a
GitHub-hosted runner every six hours. It is inert where it sits; activate it
deliberately:

```sh
cp tools/lichess/deploy/lichess-bot.yml .github/workflows/lichess-bot.yml
gh secret set LICHESS_BOT_TOKEN
```

Actions minutes are free for public repositories, but a job is capped at six
hours, so the bot plays for 5h20m, takes SIGINT, and uses the remainder to
finish games. Between runs it is offline, and GitHub disables scheduled
workflows after 60 days without repository activity. Treat it as a stopgap for
a free host, not as 24/7 hosting: GitHub's terms scope Actions to work related
to the project rather than to running a game bot around the clock.

## Troubleshooting

**"Out of host capacity"** — the free A1 shapes are oversubscribed. Try another
availability domain, retry later, or fall back to the AMD micro in the same
region. Creating the instance in the Console (not the CLI) with a retry loop is
the usual workaround.

**`setup.sh` fails on the Go version** — the installer should have replaced it;
check `go version` is ≥ 1.23.2 and that `/usr/local/bin/go` exists.

**Bot connects then games abort with timeouts** — on a 1 GB host, confirm
`Hash: 64` in `config.deploy.yml`. On the micro shape the engine is
CPU-starved; raise `engine.uci_options."Move Overhead"` or restrict
`challenge.max_base`.

**Two bots fighting over the account** — stop the laptop copy. Two processes
with the same token will both try to accept games.

**Instance reclaimed for being idle** — Oracle may reclaim Always Free
instances that sit below 20% CPU/network (and memory, for A1) for a week. A bot
that idles between games is exactly that shape; running anything else small on
the box or accepting the risk are the options.

## Files

- `install.sh` — idempotent installer for a fresh Ubuntu VM.
- `gochess-bot.service` — unit template; `install.sh` substitutes the user,
  paths, and config.
- `lichess-bot.yml` — opt-in GitHub Actions workflow (copy into
  `.github/workflows/`).
- `config.deploy.yml`, `.env`, `lichess-bot/`, `venv/`, `games/`, `*.log` — all
  generated on the host and gitignored.
