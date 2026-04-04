#!/usr/bin/env bash
set -Eeuo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "${SCRIPT_DIR}"

trap 'echo "[ERROR] Line ${LINENO}: command failed" >&2' ERR

# ---------------------------------------------------------------------------
# Step 1 — Check for Homebrew
# ---------------------------------------------------------------------------
if ! command -v brew &>/dev/null; then
    echo "Error: Homebrew is required. Install from https://brew.sh" >&2
    exit 1
fi

# ---------------------------------------------------------------------------
# Step 2 — Install dependencies via Homebrew
# (brew install is idempotent — skips already-installed packages)
# ---------------------------------------------------------------------------
echo "[dev] Installing dependencies via Homebrew..."
brew install go pkl ffmpeg node pipx

# ---------------------------------------------------------------------------
# Step 3 — Install yt-dlp via pipx with bgutil PO Token plugin
# (pipx gives yt-dlp its own isolated Python environment;
#  pipx inject adds the plugin INTO that same environment so yt-dlp finds it)
# ---------------------------------------------------------------------------
if ! pipx list 2>/dev/null | grep -q "package yt-dlp"; then
    echo "[dev] Installing yt-dlp via pipx..."
    pipx install yt-dlp
fi
if ! pipx runpip yt-dlp show bgutil-ytdlp-pot-provider &>/dev/null 2>&1; then
    echo "[dev] Injecting bgutil PO Token plugin into yt-dlp environment..."
    pipx inject yt-dlp bgutil-ytdlp-pot-provider
fi

# ---------------------------------------------------------------------------
# Step 4 — Set up PO Token server (if not already cloned)
# ---------------------------------------------------------------------------
BGUTIL_DIR="${HOME}/bgutil-ytdlp-pot-provider"
if [[ ! -d "${BGUTIL_DIR}" ]]; then
    echo "[dev] Setting up PO Token server..."
    git clone --single-branch --branch 1.3.1 \
        https://github.com/Brainicism/bgutil-ytdlp-pot-provider.git "${BGUTIL_DIR}"
    cd "${BGUTIL_DIR}/server"
    npm ci
    npx tsc
    cd "${SCRIPT_DIR}"
fi

# ---------------------------------------------------------------------------
# Step 5 — Generate Config.pkl if it does not exist
# ---------------------------------------------------------------------------
if [[ ! -f config/Config.pkl ]]; then
    echo "[dev] Config.pkl not found. Generating from template..."

    while true; do
        read -r -p "Enter Telegram bot token: " BOT_TOKEN
        [[ -n "${BOT_TOKEN}" ]] && break
        echo "Error: bot token is required" >&2
    done

    while true; do
        read -r -p "Enter admin Telegram username (without @): " ADMIN_USERNAME
        [[ -n "${ADMIN_USERNAME}" ]] && break
        echo "Error: admin username is required" >&2
    done

    while true; do
        read -r -p "Enter bot name (username without @): " BOT_NAME
        [[ -n "${BOT_NAME}" ]] && break
        echo "Error: bot name is required" >&2
    done

    YTDLP_PATH="$(which yt-dlp)"

    pkl eval \
        -p "botToken=${BOT_TOKEN}" \
        -p "adminUsername=${ADMIN_USERNAME}" \
        -p "botName=${BOT_NAME}" \
        -p "ytdlpPath=${YTDLP_PATH}" \
        config/InstallTemplate.pkl \
        -o config/Config.pkl

    echo "[dev] Config.pkl generated successfully."
fi

# ---------------------------------------------------------------------------
# Step 6 — Build the bot
# ---------------------------------------------------------------------------
echo "[dev] Building bot..."
make init && make build

# ---------------------------------------------------------------------------
# Step 7 — Start PO Token server in background, then bot in foreground
# ---------------------------------------------------------------------------
POT_SERVER_PID=""

if [[ -d "${HOME}/bgutil-ytdlp-pot-provider/server/build" ]]; then
    echo "[dev] Starting PO Token server in background (port 4416)..."
    node "${HOME}/bgutil-ytdlp-pot-provider/server/build/main.js" &
    POT_SERVER_PID=$!

    echo "[dev] Waiting for PO Token server to become ready..."
    for i in $(seq 1 30); do
        if curl -sf http://127.0.0.1:4416 >/dev/null 2>&1; then
            echo "[dev] PO Token server is ready."
            break
        fi
        if ! kill -0 "${POT_SERVER_PID}" 2>/dev/null; then
            echo "[dev] Warning: PO Token server exited unexpectedly." >&2
            POT_SERVER_PID=""
            break
        fi
        sleep 1
    done
fi

cleanup() {
    local code=$?
    if [[ -n "${POT_SERVER_PID}" ]]; then
        echo "[dev] Stopping PO Token server (PID ${POT_SERVER_PID})..."
        kill "${POT_SERVER_PID}" 2>/dev/null || true
    fi
    exit "${code}"
}
trap cleanup INT TERM EXIT

echo "[dev] Starting bot (Ctrl+C to stop)..."
./build/tg-downloader
