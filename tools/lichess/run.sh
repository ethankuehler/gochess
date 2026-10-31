#!/usr/bin/env bash
# Run gochess on Lichess through the lichess-bot bridge.
#
#   ./run.sh              # rebuild gochess, then play
#   ./run.sh --upgrade    # upgrade the account to a bot (irreversible), then exit
#
# The OAuth token is read from tools/lichess/.env, or from $LICHESS_BOT_TOKEN.
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
root="$(cd "$here/../.." && pwd)"
botdir="$here/lichess-bot"
venv="$here/venv"
env_file="$here/.env"

usage() {
  cat <<EOF
usage: $0 [--upgrade [--yes]] [lichess-bot options]

  --upgrade   upgrade the account to a Lichess bot account (irreversible)
  --yes       skip the confirmation prompt for --upgrade
  -h, --help  show this help

The token is read from $env_file (copy .env.example and fill it in),
or from an exported LICHESS_BOT_TOKEN, which takes precedence.
EOF
}

mode="run"
assume_yes=false
while [[ $# -gt 0 ]]; do
  case "$1" in
    --upgrade) mode="upgrade"; shift ;;
    --yes|-y) assume_yes=true; shift ;;
    -h|--help) usage; exit 0 ;;
    *) break ;;
  esac
done

# Load the gitignored .env file unless the token was already exported. The
# explicit environment variable wins, which makes one-off overrides easy.
if [[ -z "${LICHESS_BOT_TOKEN:-}" && -f "$env_file" ]]; then
  set -a
  # shellcheck disable=SC1090
  . "$env_file"
  set +a
fi

if [[ -z "${LICHESS_BOT_TOKEN:-}" ]]; then
  if [[ -f "$env_file" ]]; then
    echo "error: $env_file does not set LICHESS_BOT_TOKEN" >&2
    echo "edit it and add: LICHESS_BOT_TOKEN=lip_..." >&2
  else
    echo "error: no Lichess token found" >&2
    echo "copy $here/.env.example to $env_file and paste the token, or export it:" >&2
    echo '  export LICHESS_BOT_TOKEN="lip_..."' >&2
  fi
  exit 1
fi

if [[ ! -f "$botdir/lichess-bot.py" || ! -x "$venv/bin/python" ]]; then
  echo "error: lichess-bot is not installed; run $here/setup.sh first" >&2
  exit 1
fi

if [[ "$mode" == "upgrade" ]]; then
  if ! command -v curl >/dev/null 2>&1; then
    echo "error: curl not found" >&2
    exit 1
  fi
  echo "WARNING: upgrading to a bot account is irreversible."
  echo "The account will no longer be playable through the Lichess web UI."
  if [[ "$assume_yes" != true ]]; then
    read -r -p "Type 'upgrade' to continue: " answer
    if [[ "$answer" != "upgrade" ]]; then
      echo "aborted"
      exit 1
    fi
  fi
  if ! response="$(curl -fsS -X POST https://lichess.org/api/bot/account/upgrade \
    -H "Authorization: Bearer $LICHESS_BOT_TOKEN")"; then
    echo "error: upgrade request failed (is the token valid and the account unused?)" >&2
    exit 1
  fi
  printf '%s\n' "$response"
  echo
  echo "account upgraded. Start the bot with: $here/run.sh"
  exit 0
fi

# Keep Go's caches inside the ignored workspace directories; the default
# $HOME caches may not be writable in sandboxed environments.
export GOCACHE="${GOCACHE:-$root/.gocache}"
export GOMODCACHE="${GOMODCACHE:-$root/.gomodcache}"
export GOPATH="${GOPATH:-$root/.gopath}"

echo "==> building gochess"
(cd "$root" && go build -o gochess ./cmd/uci)

cd "$botdir"

# Hold a power assertion for as long as the bot runs. Lichess keeps the game
# clock running while the host is suspended, so a sleep of even a minute can
# turn a winning position into a time forfeit. On macOS, caffeinate -dimsu
# keeps the display and disk awake, prevents idle and AC sleep, and declares
# the user active. On Linux, systemd-inhibit blocks sleep and idle for the
# lifetime of the bot. Neither protects against a forced/root suspend.
#
# Nothing here needs root for ordinary sleep prevention. On macOS, caffeinate
# cannot prevent lid-close sleep, so keep the lid open. Linux lid-close behavior
# may be controlled separately by the desktop environment or logind.
bot=( "$venv/bin/python" lichess-bot.py
      --config "$here/config.yml"
      --logfile "$here/lichess-bot.log"
      "$@" )
case "$(uname -s)" in
  Darwin)
    if command -v caffeinate >/dev/null 2>&1; then
      echo "==> keeping the host awake with caffeinate"
      exec caffeinate -dimsu "${bot[@]}"
    fi
    ;;
  Linux)
    if command -v systemd-inhibit >/dev/null 2>&1; then
      echo "==> keeping the host awake with systemd-inhibit"
      exec systemd-inhibit \
        --what=sleep:idle \
        --who=gochess \
        --why="Lichess bot is running" \
        --mode=block \
        "${bot[@]}"
    fi
    ;;
esac
echo "warning: no supported power inhibitor found; the host may sleep mid-game" >&2
exec "${bot[@]}"
