#!/usr/bin/env bash
# Install gochess as a Lichess bot on a fresh Ubuntu VM (Oracle Cloud Always
# Free or any other systemd host).
#
# Run it as your normal login user (for example "ubuntu"), not as root; it uses
# sudo for the steps that need it.
#
#   tools/lichess/deploy/install.sh               # 12 GB host: Hash 256
#   tools/lichess/deploy/install.sh --low-ram     # 1 GB host: Hash 64
#   tools/lichess/deploy/install.sh --no-start    # install, do not start yet
#
# Safe to re-run: it updates lichess-bot, rebuilds gochess, and restarts the
# service. An existing config.deploy.yml and .env are left alone.
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo="$(cd "$here/../../.." && pwd)"

low_ram=false
token=""
token_file=""
defer_token=false
start_service=true

usage() {
  cat <<EOF
usage: $0 [--low-ram] [--token TOKEN | --token-file FILE | --no-token] [--no-start]

  --low-ram         tune the generated config for a 1 GB host (Hash 64 instead
                    of 256). Use this on an AMD micro instance; the A1 shape
                    has room for the default.
  --token T         write T to tools/lichess/.env instead of prompting. This
                    puts the token in shell history; prefer --token-file.
  --token-file F    read the token from F (one line, optional trailing
                    newline) and write it to tools/lichess/.env.
  --no-token        install without a token and leave the service stopped.
                    Useful when the install is driven remotely; write .env
                    afterwards and start the unit by hand.
  --no-start        install and enable the unit but do not start it yet.

Run from a clone of the repo on the VM. The script installs Go if the system
one is older than go.mod requires, then runs tools/lichess/setup.sh.
EOF
}

die() { echo "error: $*" >&2; exit 1; }
note() { printf '==> %s\n' "$*"; }

while [[ $# -gt 0 ]]; do
  case "$1" in
    --low-ram) low_ram=true; shift ;;
    --token)
      [[ $# -ge 2 ]] || die "--token needs a value"
      token="$2"
      shift 2
      ;;
    --token-file)
      [[ $# -ge 2 ]] || die "--token-file needs a value"
      token_file="$2"
      shift 2
      ;;
    --no-token) defer_token=true; shift ;;
    --no-start) start_service=false; shift ;;
    -h|--help) usage; exit 0 ;;
    *) usage >&2; die "unknown argument: $1" ;;
  esac
done

[[ $EUID -ne 0 ]] || die "run this as your normal user, not root; it will sudo when needed"
[[ -f "$repo/go.mod" ]] || die "$repo does not look like the gochess repo"
[[ -d /run/systemd/system ]] || die "systemd is not running; this script targets a normal Ubuntu VM"

lichess_dir="$repo/tools/lichess"
botdir="$lichess_dir/lichess-bot"
venv="$lichess_dir/venv"
env_file="$lichess_dir/.env"
config="$lichess_dir/config.yml"
deploy_config="$lichess_dir/config.deploy.yml"
logfile="$lichess_dir/lichess-bot.log"
unit_path="/etc/systemd/system/gochess-bot.service"
required_go="1.23.2"

# 1. System packages ---------------------------------------------------------
note "installing system packages"
sudo apt-get update -qq
sudo DEBIAN_FRONTEND=noninteractive apt-get install -y -qq \
  git curl ca-certificates python3 python3-venv tar

# 2. Go ----------------------------------------------------------------------
# go.mod requires 1.23.2. Ubuntu 24.04 ships 1.22, so install upstream Go when
# the system one is missing or too old.
have_go=""
if command -v go >/dev/null 2>&1; then
  have_go="$(go env GOVERSION 2>/dev/null | sed 's/^go//')"
fi
if [[ -z "$have_go" ]] ||
  [[ "$(printf '%s\n%s\n' "$required_go" "$have_go" | sort -V | head -n1)" != "$required_go" ]]; then
  case "$(uname -m)" in
    x86_64) goarch="amd64" ;;
    aarch64 | arm64) goarch="arm64" ;;
    *) die "unsupported architecture: $(uname -m)" ;;
  esac
  go_version="${GO_VERSION:-}"
  if [[ -z "$go_version" ]]; then
    go_version="$(curl -fsSL 'https://go.dev/VERSION?m=text' 2>/dev/null | head -n1 || true)"
  fi
  go_version="${go_version#go}"
  # Fall back to a known-good release if go.dev is unreachable or answers oddly.
  [[ "$go_version" =~ ^1\.[0-9]+(\.[0-9]+)?$ ]] || go_version="1.27.2"
  note "installing Go $go_version for linux/$goarch (system Go: ${have_go:-none})"
  tmp="$(mktemp -d)"
  trap 'rm -rf "$tmp"' EXIT
  curl -fsSL "https://go.dev/dl/go${go_version}.linux-${goarch}.tar.gz" -o "$tmp/go.tar.gz"
  sudo rm -rf /usr/local/go
  sudo tar -C /usr/local -xzf "$tmp/go.tar.gz"
  rm -rf "$tmp"
  trap - EXIT
  # /usr/local/bin is on the default PATH for non-login shells too.
  sudo ln -sf /usr/local/go/bin/go /usr/local/bin/go
  sudo ln -sf /usr/local/go/bin/gofmt /usr/local/bin/gofmt
fi
note "using $(go version)"

# 3. Bridge and engine -------------------------------------------------------
# setup.sh clones or updates lichess-bot, creates the virtualenv, installs its
# Python dependencies, builds ./gochess, and validates config.yml.
note "installing lichess-bot and building gochess"
"$lichess_dir/setup.sh"

# 4. OAuth token -------------------------------------------------------------
have_token=false
if grep -Eq '^LICHESS_BOT_TOKEN=.+' "$env_file" 2>/dev/null; then
  have_token=true
fi

if ! $have_token; then
  if [[ -n "$token_file" ]]; then
    [[ -r "$token_file" ]] || die "cannot read token file: $token_file"
    token="$(tr -d '[:space:]' <"$token_file")"
    [[ -n "$token" ]] || die "token file is empty: $token_file"
  fi
  if [[ -z "$token" ]] && ! $defer_token; then
    # Only prompt on a real terminal; over SSH with a piped script there is
    # nothing to read, so fail loudly instead of hanging.
    [[ -t 0 ]] || die "no token: use --token-file FILE, --token T, or --no-token"
    printf 'Lichess token (bot:play scope, input hidden): '
    IFS= read -r -s token
    printf '\n'
  fi
  if [[ -n "$token" ]]; then
    (umask 077 && printf 'LICHESS_BOT_TOKEN=%s\n' "$token" >"$env_file")
    have_token=true
  fi
fi

if $have_token; then
  chmod 600 "$env_file"
else
  # Starting without a token would only crash-loop the unit.
  note "no token yet; the service will be installed but left stopped"
  start_service=false
fi

# 5. Per-host config ---------------------------------------------------------
# Kept out of git (see .gitignore) and derived from config.yml so there is only
# one config to maintain. Delete it and re-run to pick up config.yml changes.
if [[ ! -f "$deploy_config" ]]; then
  cp "$config" "$deploy_config"
  if $low_ram; then
    note "tuning for a small host: Hash 64"
    # Rewrite through a temp file instead of `sed -i`: BSD and GNU sed disagree
    # about -i, and this keeps the script portable.
    sed 's/^\([[:space:]]*\)Hash: 256$/\1Hash: 64/' "$deploy_config" >"$deploy_config.tmp"
    mv "$deploy_config.tmp" "$deploy_config"
    grep -q 'Hash: 64' "$deploy_config" ||
      die "could not patch Hash in $deploy_config; edit it by hand"
  fi
fi

# 6. systemd unit ------------------------------------------------------------
note "installing $unit_path"
sed -e "s|@USER@|$(id -un)|g" \
  -e "s|@GROUP@|$(id -gn)|g" \
  -e "s|@BOTDIR@|$botdir|g" \
  -e "s|@ENVFILE@|$env_file|g" \
  -e "s|@VENV@|$venv|g" \
  -e "s|@CONFIG@|$deploy_config|g" \
  -e "s|@LOGFILE@|$logfile|g" \
  "$here/gochess-bot.service" | sudo tee "$unit_path" >/dev/null

# lichess-bot appends to its logfile forever; this keeps it bounded.
sudo tee /etc/logrotate.d/gochess-bot >/dev/null <<EOF
$logfile {
    weekly
    rotate 4
    compress
    delaycompress
    missingok
    notifempty
    copytruncate
}
EOF

sudo systemctl daemon-reload
sudo systemctl enable gochess-bot.service >/dev/null
if $start_service; then
  note "starting gochess-bot"
  sudo systemctl restart gochess-bot.service
else
  note "not starting; run: sudo systemctl start gochess-bot"
fi

cat <<EOF

Done. Useful commands on this host:

  systemctl status gochess-bot
  journalctl -u gochess-bot -f
  tail -f $logfile
  sudo systemctl stop gochess-bot     # finishes running games first

EOF

if ! $have_token; then
  cat <<EOF
No token was installed, so the unit is stopped. To finish, write the token and
start it (the prompt is hidden, so it never reaches your shell history):

  read -rsp 'Lichess token: ' T; echo
  umask 077; printf 'LICHESS_BOT_TOKEN=%s\n' "\$T" > '$env_file'
  sudo systemctl start gochess-bot

EOF
fi

cat <<EOF
The bot appears as https://lichess.org/@/gochess6767 once it connects.
EOF
