#!/usr/bin/env bash
set -Eeuo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

trap 'echo "[ERROR] Line ${LINENO}: command failed" >&2' ERR

# ---------------------------------------------------------------------------
# Root check
# ---------------------------------------------------------------------------
[[ $EUID -ne 0 ]] && echo "Error: run as root" >&2 && exit 1

# ---------------------------------------------------------------------------
# Repository URL (change this if you fork the repo)
# ---------------------------------------------------------------------------
REPO_URL="https://github.com/SamirAkhmedov/tg-downloader.git"

# ---------------------------------------------------------------------------
# --update mode: pull latest code, rebuild, restart services
# ---------------------------------------------------------------------------
if [[ "${1:-}" == "--update" ]]; then
    echo "[update] Pulling latest code..."
    sudo -u tgbot bash -c "
        export PATH=/usr/local/go/bin:/usr/local/bin:\$PATH
        export HOME=/home/tgbot
        cd /home/tgbot/tg-downloader
        git pull
        make init && make build
    "
    echo "[update] Restarting services..."
    systemctl restart tg-downloader.service pot-server.service
    echo "[update] Done. Services restarted successfully."
    exit 0
fi

echo "================================================================"
echo "  tg-downloader VPS Install Script"
echo "  Target: Ubuntu/Debian"
echo "================================================================"
echo

# ---------------------------------------------------------------------------
# Step 1 — System prerequisites
# ---------------------------------------------------------------------------
echo "[1/16] Installing system prerequisites..."
export DEBIAN_FRONTEND=noninteractive
apt-get update
apt-get install -y --no-install-recommends \
    curl ca-certificates gcc libc6-dev git make \
    python3 python3-pip pipx ffmpeg

# ---------------------------------------------------------------------------
# Step 2 — Install Go (tarball from go.dev)
# ---------------------------------------------------------------------------
echo "[2/16] Installing Go..."
GO_VERSION="1.25.1"
curl -LO "https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz"
rm -rf /usr/local/go
tar -C /usr/local -xzf "go${GO_VERSION}.linux-amd64.tar.gz"
rm "go${GO_VERSION}.linux-amd64.tar.gz"
export PATH=/usr/local/go/bin:$PATH
echo 'export PATH=/usr/local/go/bin:$PATH' > /etc/profile.d/golang.sh

# ---------------------------------------------------------------------------
# Step 3 — Install Pkl CLI (pinned to 0.28.0 matching Dockerfile ARG)
# ---------------------------------------------------------------------------
PKL_VERSION="0.28.0"
echo "[3/16] Installing Pkl CLI ${PKL_VERSION}..."
curl -L -o /usr/local/bin/pkl \
    "https://github.com/apple/pkl/releases/download/${PKL_VERSION}/pkl-linux-amd64"
chmod +x /usr/local/bin/pkl

# ---------------------------------------------------------------------------
# Step 4 — Install Node.js 20 LTS via NodeSource
# (apt default ships EOL Node.js 12 on Ubuntu 22.04 — unusable for bgutil)
# ---------------------------------------------------------------------------
echo "[4/16] Installing Node.js 20 LTS via NodeSource..."
curl -fsSL https://deb.nodesource.com/setup_20.x | bash -
apt-get install -y nodejs

# ---------------------------------------------------------------------------
# Step 5 — Create tgbot system user
# ---------------------------------------------------------------------------
echo "[5/16] Creating tgbot system user..."
if ! id tgbot &>/dev/null; then
    useradd -m -s /bin/bash tgbot
    echo "  Created user: tgbot"
else
    echo "  User tgbot already exists — skipping"
fi

# ---------------------------------------------------------------------------
# Step 6 — Install yt-dlp via pipx as tgbot user
# (pipx isolates from system Python — avoids PEP 668 issues on Ubuntu 24.04)
# ---------------------------------------------------------------------------
echo "[6/16] Installing yt-dlp and bgutil plugin via pipx..."
sudo -u tgbot bash -c "
    export HOME=/home/tgbot
    pipx install yt-dlp
    pipx inject yt-dlp bgutil-ytdlp-pot-provider
"

# ---------------------------------------------------------------------------
# Step 7 — Clone the tg-downloader repo as tgbot user
# ---------------------------------------------------------------------------
echo "[7/16] Cloning tg-downloader repository..."
if [[ ! -d /home/tgbot/tg-downloader ]]; then
    sudo -u tgbot git clone "${REPO_URL}" /home/tgbot/tg-downloader
    echo "  Cloned to /home/tgbot/tg-downloader"
else
    echo "  /home/tgbot/tg-downloader already exists — skipping clone"
fi

# ---------------------------------------------------------------------------
# Step 8 — Clone and build bgutil PO Token server as tgbot user
# ---------------------------------------------------------------------------
echo "[8/16] Setting up bgutil PO Token server (branch 1.3.1)..."
if [[ ! -d /home/tgbot/bgutil-ytdlp-pot-provider ]]; then
    sudo -u tgbot bash -c "
        cd /home/tgbot
        git clone --single-branch --branch 1.3.1 \
            https://github.com/Brainicism/bgutil-ytdlp-pot-provider.git
        cd bgutil-ytdlp-pot-provider/server
        npm ci
        npx tsc
    "
    echo "  bgutil server built at /home/tgbot/bgutil-ytdlp-pot-provider/server/build/"
else
    echo "  /home/tgbot/bgutil-ytdlp-pot-provider already exists — skipping clone"
fi

# ---------------------------------------------------------------------------
# Step 9 — Prompt for user configuration values
# ---------------------------------------------------------------------------
echo
echo "[9/16] Collecting configuration..."
echo

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

# ---------------------------------------------------------------------------
# Step 10 — Generate Config.pkl using Pkl templating
# ---------------------------------------------------------------------------
echo "[10/16] Generating Config.pkl from template..."
YTDLP_PATH=$(sudo -u tgbot bash -c 'export HOME=/home/tgbot && echo $HOME/.local/bin/yt-dlp')

sudo -u tgbot bash -c "
    export PATH=/usr/local/go/bin:/usr/local/bin:\$PATH
    export HOME=/home/tgbot
    cd /home/tgbot/tg-downloader
    pkl eval \
        -p \"botToken=${BOT_TOKEN}\" \
        -p \"adminUsername=${ADMIN_USERNAME}\" \
        -p \"botName=${BOT_NAME}\" \
        -p \"ytdlpPath=${YTDLP_PATH}\" \
        config/InstallTemplate.pkl \
        -o config/Config.pkl
"
echo "  Config.pkl generated at /home/tgbot/tg-downloader/config/Config.pkl"

# ---------------------------------------------------------------------------
# Step 11 — Build the bot
# ---------------------------------------------------------------------------
echo "[11/16] Building tg-downloader (make init && make build)..."
sudo -u tgbot bash -c "
    export PATH=/usr/local/go/bin:/usr/local/bin:\$PATH
    export HOME=/home/tgbot
    cd /home/tgbot/tg-downloader
    make init && make build
"
echo "  Binary built at /home/tgbot/tg-downloader/build/tg-downloader"

# ---------------------------------------------------------------------------
# Step 12 — Create systemd service for tg-downloader
# ---------------------------------------------------------------------------
echo "[12/16] Creating systemd service: tg-downloader.service..."
cat > /etc/systemd/system/tg-downloader.service <<'EOF'
[Unit]
Description=Telegram Downloader Bot
After=network.target
StartLimitIntervalSec=60
StartLimitBurst=5

[Service]
Type=simple
User=tgbot
WorkingDirectory=/home/tgbot/tg-downloader
ExecStart=/home/tgbot/tg-downloader/build/tg-downloader
Restart=on-failure
RestartSec=5
StandardOutput=journal
StandardError=journal
Environment=PATH=/home/tgbot/.local/bin:/usr/local/go/bin:/usr/local/bin:/usr/bin:/bin

[Install]
WantedBy=multi-user.target
EOF

# ---------------------------------------------------------------------------
# Step 13 — Create systemd service for PO Token server
# (Independent of tg-downloader — bot works even if PO server is down)
# ---------------------------------------------------------------------------
echo "[13/16] Creating systemd service: pot-server.service..."
cat > /etc/systemd/system/pot-server.service <<'EOF'
[Unit]
Description=bgutil PO Token Server
After=network.target
StartLimitIntervalSec=60
StartLimitBurst=5

[Service]
Type=simple
User=tgbot
WorkingDirectory=/home/tgbot/bgutil-ytdlp-pot-provider/server
ExecStart=/usr/bin/node build/main.js
Restart=on-failure
RestartSec=5
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
EOF

# ---------------------------------------------------------------------------
# Step 14 — Enable and start services
# ---------------------------------------------------------------------------
echo "[14/16] Enabling and starting services..."
systemctl daemon-reload
systemctl enable tg-downloader.service pot-server.service
systemctl start tg-downloader.service pot-server.service

# ---------------------------------------------------------------------------
# Step 15 — Install yt-dlp auto-update cron job (weekly Sunday 3:00 AM)
# ---------------------------------------------------------------------------
echo "[15/16] Installing weekly yt-dlp auto-update cron job..."
PIPX_PATH=$(sudo -u tgbot bash -c 'export HOME=/home/tgbot && which pipx')
YTDLP_CRON="0 3 * * 0 ${PIPX_PATH} upgrade yt-dlp"
(crontab -u tgbot -l 2>/dev/null || true; echo "${YTDLP_CRON}") \
    | awk '!x[$0]++' \
    | crontab -u tgbot -
echo "  Cron job installed: ${YTDLP_CRON}"

# ---------------------------------------------------------------------------
# Step 16 — Print summary
# ---------------------------------------------------------------------------
echo
echo "================================================================"
echo "  Installation Complete!"
echo "================================================================"
echo
echo "Installed versions:"
echo "  Go:     $(go version 2>/dev/null || echo 'not found')"
echo "  Pkl:    $(pkl --version 2>/dev/null || echo 'not found')"
echo "  yt-dlp: $(sudo -u tgbot bash -c 'export HOME=/home/tgbot && export PATH=/home/tgbot/.local/bin:$PATH && yt-dlp --version 2>/dev/null' || echo 'not found')"
echo "  Node:   $(node --version 2>/dev/null || echo 'not found')"
echo "  ffmpeg: $(ffmpeg -version 2>/dev/null | head -1 || echo 'not found')"
echo
echo "Service status:"
systemctl is-active tg-downloader.service && echo "  tg-downloader: running" || echo "  tg-downloader: stopped (check logs)"
systemctl is-active pot-server.service    && echo "  pot-server:    running" || echo "  pot-server:    stopped (check logs)"
echo
echo "Cron jobs for tgbot user:"
crontab -u tgbot -l 2>/dev/null || echo "  (none)"
echo
echo "View bot logs:         journalctl -u tg-downloader.service -f"
echo "View PO Token logs:    journalctl -u pot-server.service -f"
echo "Update bot:            sudo bash ${SCRIPT_DIR}/install.sh --update"
echo
