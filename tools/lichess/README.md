# gochess on Lichess

This directory wires `./gochess` into the [Lichess Bot API](https://lichess.org/api#tag/Bot)
through the third-party [lichess-bot](https://github.com/lichess-bot-devs/lichess-bot)
bridge. It is optional development tooling: no Go package imports network code,
and CI never runs it.

The point is to get a real Lichess Glicko-2 rating by playing rated games
against other bots. That is a slow external check, not a replacement for
`cmd/arena` as a per-commit Elo benchmark.

## One-time setup

Prerequisites: Go, Python 3.10+, git, and a **fresh** Lichess account. An
account that has already played games cannot be upgraded to a bot account, and
the upgrade is irreversible.

1. Create the account, then create a personal API token with the `bot:play`
   scope:

   <https://lichess.org/account/oauth/token/create?scopes[]=bot:play>

   Keep the token out of the repository. `run.sh` reads it from
   `tools/lichess/.env` or the environment.

2. Install the bridge and build the engine:

   ```sh
   ./tools/lichess/setup.sh
   ```

   This clones lichess-bot into `tools/lichess/lichess-bot/`, creates a
   virtualenv in `tools/lichess/venv/`, installs its Python dependencies, builds
   `./gochess`, and validates `config.yml`. It uses the network; the Go build
   and test suite do not.

3. Save the token in the gitignored `.env` file:

   ```sh
   cp tools/lichess/.env.example tools/lichess/.env
   # edit tools/lichess/.env and paste the token after LICHESS_BOT_TOKEN=
   ```

   An exported `LICHESS_BOT_TOKEN` overrides the file, so one-off runs can
   still use `LICHESS_BOT_TOKEN=... ./tools/lichess/run.sh`.

4. Upgrade the account to a bot account. This is irreversible:

   ```sh
   ./tools/lichess/run.sh --upgrade
   ```

   Pass `--yes` to skip the confirmation prompt in scripts.

5. Start the bot:

   ```sh
   ./tools/lichess/run.sh
   ```

   `run.sh` rebuilds `./gochess` before each start, so the running bot always
   matches the working tree.

## What it plays

`config.yml` is set up for:

- **Rated games against other bots** via matchmaking, plus **casual challenges
  from humans**. Humans can challenge
  [gochess6767](https://lichess.org/@/gochess6767); Lichess only offers casual
  challenges against bots.
- **Two slots:** one is reserved for a human challenger, the other is used for
  rated bot matchmaking. A human can therefore get a game even while a rated
  bot game is running.
- Standard chess, blitz or rapid, from **3+2 to 10+5**. At least a **+2
  increment** is required; the 5s-per-move cap relies on it.
- After a bot game ends, it waits one minute and challenges another bot near its
  rating.
- No resignation and no draw offers, because gochess's evaluation is not yet
  reliable enough to make those calls.
- PGNs in `tools/lichess/games/` and a log in `tools/lichess/lichess-bot.log`.

Lichess limits a bot to **100 rated games per day against other bots**; casual
human games do not count against that. Expect several days of running for a
rating that is not provisional. The one-minute matchmaking idle timeout
intentionally uses that allowance, so `setup.sh` prints a warning about the cap;
it is expected.

## Reading the rating

Load the token from `.env` first (skip this if you exported it):

```sh
set -a; . tools/lichess/.env; set +a
```

Then query the account:

```sh
curl -s -H "Authorization: Bearer $LICHESS_BOT_TOKEN" https://lichess.org/api/account \
  | python3 -c 'import json,sys; p=json.load(sys.stdin)["perfs"]["blitz"]; print(p["rating"], "provisional" if p.get("prov") else "established")'
```

`perfs` also has `bullet`, `rapid`, `classical`, and variant entries as they are
played. The number is Glicko-2 on Lichess's bot pool, not an Elo on Stockfish's
`UCI_Elo` scale; do not compare it directly with `cmd/arena` output.

## Stopping, updating, cleaning up

- **Stop:** Ctrl-C. With `quit_after_all_games_finish: true`, the current games
  finish first.
- **Update:** re-run `./tools/lichess/setup.sh`. It pulls lichess-bot's master
  branch and rebuilds gochess.
- **Remove everything:** delete `tools/lichess/lichess-bot/`,
  `tools/lichess/venv/`, `tools/lichess/games/`, and
  `tools/lichess/lichess-bot.log`. Those paths are gitignored.

## Troubleshooting

- `LICHESS_BOT_TOKEN is not set` / `no Lichess token found` — fill in
  `tools/lichess/.env`, or export the token in the same shell that runs
  `run.sh`.
- `upgrade request failed` — the token is invalid, or the account has played
  games and can no longer become a bot.
- No games are starting — check `lichess-bot.log`. The bot pool at a given
  rating can be sparse; `challenge.rating_difference` and
  `matchmaking.opponent_rating_difference` are deliberately wide (500) to find
  opponents while the rating is unsettled.
- The engine times out — raise `move_overhead` or `engine.uci_options.Move Overhead`,
  or restrict `challenge.max_base` to shorter games.

## Files

- `config.yml` — tokenless bridge configuration; safe to commit.
- `.env.example` — template for the token file; safe to commit.
- `.env` — your actual token; gitignored, never commit it.
- `setup.sh` — installs the bridge, builds gochess, validates the config.
- `run.sh` — loads `.env`, optionally upgrades the account, rebuilds gochess,
  and runs the bot.
- `lichess-bot/`, `venv/`, `games/`, `*.log` — generated locally and gitignored.
