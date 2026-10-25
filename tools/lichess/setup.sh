#!/usr/bin/env bash
# Install the third-party lichess-bot bridge and build gochess.
#
# This is local development tooling: it is not imported by the Go build, and CI
# never runs it. It does use the network to clone lichess-bot and install its
# Python dependencies.
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
root="$(cd "$here/../.." && pwd)"
botdir="$here/lichess-bot"
venv="$here/venv"
python="${PYTHON:-python3}"

if ! command -v "$python" >/dev/null 2>&1; then
  echo "error: $python not found; install Python 3.10 or newer" >&2
  exit 1
fi
if ! "$python" -c 'import sys; sys.exit(0 if sys.version_info >= (3, 10) else 1)'; then
  echo "error: lichess-bot needs Python 3.10 or newer ($python is older)" >&2
  exit 1
fi
if ! command -v git >/dev/null 2>&1; then
  echo "error: git not found" >&2
  exit 1
fi
if ! command -v go >/dev/null 2>&1; then
  echo "error: go not found" >&2
  exit 1
fi

if [[ ! -d "$botdir/.git" ]]; then
  echo "==> cloning lichess-bot"
  git clone --depth 1 https://github.com/lichess-bot-devs/lichess-bot.git "$botdir"
else
  echo "==> updating lichess-bot"
  git -C "$botdir" pull --ff-only
fi

if [[ ! -x "$venv/bin/python" ]]; then
  echo "==> creating virtualenv"
  "$python" -m venv "$venv"
fi
echo "==> installing lichess-bot dependencies"
"$venv/bin/python" -m pip install --upgrade pip
"$venv/bin/python" -m pip install -r "$botdir/requirements.txt"

# Keep Go's caches inside the ignored workspace directories; the default
# $HOME caches may not be writable in sandboxed environments.
export GOCACHE="${GOCACHE:-$root/.gocache}"
export GOMODCACHE="${GOMODCACHE:-$root/.gomodcache}"
export GOPATH="${GOPATH:-$root/.gopath}"

echo "==> building gochess"
(cd "$root" && go build -o gochess ./cmd/uci)

mkdir -p "$here/games"

echo "==> validating config"
if ! (cd "$botdir" && LICHESS_BOT_TOKEN=setup-validation "$venv/bin/python" -c \
  'from lib.config import load_config; load_config("../config.yml")'); then
  echo "error: config validation failed" >&2
  exit 1
fi

cat <<EOF

setup complete. Next:

  1. Create a fresh Lichess account. An account that has already played games
     cannot be upgraded to a bot.
  2. Create a token with the bot:play scope:
     https://lichess.org/account/oauth/token/create?scopes[]=bot:play
  3. cp "$here/.env.example" "$here/.env"
     # then paste the token into .env (or export LICHESS_BOT_TOKEN)
  4. $here/run.sh --upgrade    # once, irreversible
  5. $here/run.sh

The bot plays rated games against other bots and writes PGNs to
$here/games and a log to $here/lichess-bot.log.
EOF
