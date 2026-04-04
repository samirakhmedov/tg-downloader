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

    # Ensure yt-dlp system config exists (backfill for pre-fix installations)
    if [[ ! -f /etc/yt-dlp.conf ]]; then
        echo "[update] Creating /etc/yt-dlp.conf..."
        cat > /etc/yt-dlp.conf << 'EOF'
# System-wide yt-dlp configuration for tg-downloader
--remote-components ejs:github
EOF
    fi

    # Ensure deno is installed (backfill for pre-fix installations)
    if ! command -v deno &>/dev/null; then
        echo "[update] Installing Deno runtime..."
        curl -fsSL https://deno.land/install.sh | DENO_INSTALL=/usr/local sh
    fi

    # Warn if Config.pkl is missing amends directive (broken by old install script)
    CONFIG_FILE="/home/tgbot/tg-downloader/config/Config.pkl"
    if [[ -f "$CONFIG_FILE" ]] && ! head -1 "$CONFIG_FILE" | grep -q '^amends'; then
        echo
        echo "[update] WARNING: Config.pkl is missing 'amends' directive."
        echo "[update] This was caused by a bug in the original install script."
        echo "[update] Please re-run the full install script to regenerate Config.pkl,"
        echo "[update] or manually add this line at the top of ${CONFIG_FILE}:"
        echo '[update]   amends "templates/TgDownloaderConfig.pkl"'
        echo
    fi

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
echo "[1/19] Installing system prerequisites..."
export DEBIAN_FRONTEND=noninteractive
apt-get update
apt-get install -y --no-install-recommends \
    curl ca-certificates gcc libc6-dev git make \
    python3 python3-pip pipx ffmpeg unzip

# ---------------------------------------------------------------------------
# Step 2 — Install Go (tarball from go.dev)
# ---------------------------------------------------------------------------
echo "[2/19] Installing Go..."
GO_VERSION="1.25.1"
curl -LO "https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz"
rm -rf /usr/local/go
tar -C /usr/local -xzf "go${GO_VERSION}.linux-amd64.tar.gz"
rm "go${GO_VERSION}.linux-amd64.tar.gz"
export PATH=/usr/local/go/bin:$PATH
echo 'export PATH=/usr/local/go/bin:$PATH' > /etc/profile.d/golang.sh

# ---------------------------------------------------------------------------
# Step 3 — Install Pkl CLI (pinned to 0.31.1)
# ---------------------------------------------------------------------------
PKL_VERSION="0.31.1"
echo "[3/19] Installing Pkl CLI ${PKL_VERSION}..."
curl -L -o /usr/local/bin/pkl \
    "https://github.com/apple/pkl/releases/download/${PKL_VERSION}/pkl-linux-amd64"
chmod +x /usr/local/bin/pkl

# ---------------------------------------------------------------------------
# Step 4 — Install Node.js 20 LTS via NodeSource
# (apt default ships EOL Node.js 12 on Ubuntu 22.04 — unusable for bgutil)
# ---------------------------------------------------------------------------
echo "[4/19] Installing Node.js 20 LTS via NodeSource..."
curl -fsSL https://deb.nodesource.com/setup_20.x | bash -
apt-get install -y nodejs

# ---------------------------------------------------------------------------
# Step 5 — Install Deno runtime (required for YouTube JS challenge solving)
# ---------------------------------------------------------------------------
echo "[5/19] Installing Deno runtime..."
if ! command -v deno &>/dev/null; then
    curl -fsSL https://deno.land/install.sh | DENO_INSTALL=/usr/local sh
    echo "  Deno installed to /usr/local/bin/deno"
else
    echo "  Deno already installed — skipping"
fi

# ---------------------------------------------------------------------------
# Step 6 — Create tgbot system user
# ---------------------------------------------------------------------------
echo "[6/19] Creating tgbot system user..."
if ! id tgbot &>/dev/null; then
    useradd -m -s /bin/bash tgbot
    echo "  Created user: tgbot"
else
    echo "  User tgbot already exists — skipping"
fi

# ---------------------------------------------------------------------------
# Step 7 — Install yt-dlp via pipx as tgbot user
# (pipx isolates from system Python — avoids PEP 668 issues on Ubuntu 24.04)
# ---------------------------------------------------------------------------
echo "[7/19] Installing yt-dlp and bgutil plugin via pipx..."
sudo -u tgbot bash -c "
    export HOME=/home/tgbot
    pipx install yt-dlp
    pipx inject yt-dlp bgutil-ytdlp-pot-provider
"

# ---------------------------------------------------------------------------
# Step 8 — Clone the tg-downloader repo as tgbot user
# ---------------------------------------------------------------------------
echo "[8/19] Cloning tg-downloader repository..."
if [[ ! -d /home/tgbot/tg-downloader ]]; then
    sudo -u tgbot git clone "${REPO_URL}" /home/tgbot/tg-downloader
    echo "  Cloned to /home/tgbot/tg-downloader"
else
    echo "  /home/tgbot/tg-downloader already exists — skipping clone"
fi

# ---------------------------------------------------------------------------
# Step 9 — Clone and build bgutil PO Token server as tgbot user
# ---------------------------------------------------------------------------
echo "[9/19] Setting up bgutil PO Token server (branch 1.3.1)..."
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
# Step 10 — Prompt for user configuration values
# ---------------------------------------------------------------------------
echo
echo "[10/19] Collecting configuration..."
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
# Step 11 — Generate Config.pkl
# Uses heredoc + sed to preserve the 'amends' directive that pkl-go requires
# at runtime for type resolution. (pkl eval -o strips amends)
# ---------------------------------------------------------------------------
echo "[11/19] Generating Config.pkl..."
YTDLP_PATH="/home/tgbot/.local/bin/yt-dlp"

sudo -u tgbot tee /home/tgbot/tg-downloader/config/Config.pkl > /dev/null << 'PKLEOF'
amends "templates/TgDownloaderConfig.pkl"

telegramConfiguration {
    tgBotApiKey = "__BOT_TOKEN__"
    updateTimeout = 2
    updateOffset = 0
    updateLimit = 2
    botName = "__BOT_NAME__"
}

authConfiguration {
    admininstrators {
        new {
            userName = "__ADMIN_USERNAME__"
        }
    }
}

commandConfiguration {
    commands {
        ["activateGroup"] {
            command = "/a"
            description = "Activate this group for downloading"
            accessLevel = "user"
        }
        ["deactivateGroup"] {
            command = "/d"
            description = "Deactivate this group"
            accessLevel = "user"
        }
        ["getBotCommands"] {
            command = "/i"
            description = "Get available bot commands"
            accessLevel = "user"
        }
        ["loadResource"] {
            command = "/l"
            description = "Load resource for downloading"
            accessLevel = "user"
        }
        ["start"] {
            command = "/start"
            description = "Start the bot"
            accessLevel = "user"
        }
        ["getServerLoad"] {
            command = "/l"
            description = "Get server load information"
            accessLevel = "admin"
        }
        ["getAllGroups"] {
            command = "/a"
            description = "Get all groups"
            accessLevel = "admin"
        }
        ["deleteGroup"] {
            command = "/d"
            description = "Delete a group"
            accessLevel = "admin"
        }
    }

    supportedLinks {
        new {
            name = "Instagram"
            pattern = "^https://(?:www\\.)?instagram\\.com/(?:reel|p)/[A-Za-z0-9_-]+/?(?:\\?.*)?$"
            example = "-"
            strategies {
                new { name = "best"; format = "b" }
                new { name = "best 720p"; format = "best[height<=720]" }
                new { name = "worst"; format = "worst" }
            }
            extractorArgs {}
        }
        new {
            name = "TikTok"
            pattern = "^https://(?:www\\.)?vt\\.tiktok\\.com/[A-Za-z0-9]+/?(?:\\?.*)?$"
            example = "-"
            strategies {
                new { name = "best"; format = "b" }
                new { name = "best 720p"; format = "best[height<=720]" }
                new { name = "worst"; format = "worst" }
            }
            extractorArgs {}
        }
        new {
            name = "TikTok Full"
            pattern = "^https://(?:www\\.)?tiktok\\.com/@[A-Za-z0-9_.]+/(?:video|photo)/[0-9]+/?(?:\\?.*)?$"
            example = "-"
            strategies {
                new { name = "best"; format = "b" }
                new { name = "worst"; format = "worst" }
            }
            extractorArgs {}
        }
        new {
            name = "YouTube Shorts"
            pattern = "^https://(?:www\\.)?youtube\\.com/shorts/[A-Za-z0-9_-]+/?(?:\\?.*)?$"
            example = "-"
            strategies {
                new { name = "best"; format = "b" }
                new { name = "best 720p"; format = "best[height<=720]" }
                new { name = "worst"; format = "worst" }
            }
            extractorArgs { "youtube:player_client=web" }
        }
        new {
            name = "Twitter/X"
            pattern = "^https://(?:www\\.)?(vxtwitter|x|twitter)\\.com/[A-Za-z0-9_]+/status/[0-9]+(?:\\?.*)?$"
            example = "-"
            strategies {
                new { name = "best"; format = "b" }
                new { name = "best 720p"; format = "best[height<=720]" }
                new { name = "worst"; format = "worst" }
            }
            extractorArgs {}
        }
    }
}

commonDownloaderConfiguration {
    ytdlpExecutablePath = "__YTDLP_PATH__"

    forceIPv4 = true

    sleepInterval = 1.0
    maxSleepInterval = 3.0

    extractorArgs {}

    customHeaders {}
}

videoDownloaderConfiguration {
    outputFormat = "mp4"
    maxFileSizeMB = 50
    downloadTimeoutSeconds = 300
    uploadTimeoutSeconds = 300
}

workerConfiguration {
    workerCount = 2
    taskPollingInterval = 1
    maxRetries = 2
}

poTokenConfiguration {
    providerUrl = "http://127.0.0.1:4416"
    healthCheckIntervalSeconds = 900
}

debug = false
PKLEOF

# Substitute user-provided values into Config.pkl
sed -i \
    -e "s|__BOT_TOKEN__|${BOT_TOKEN}|g" \
    -e "s|__ADMIN_USERNAME__|${ADMIN_USERNAME}|g" \
    -e "s|__BOT_NAME__|${BOT_NAME}|g" \
    -e "s|__YTDLP_PATH__|${YTDLP_PATH}|g" \
    /home/tgbot/tg-downloader/config/Config.pkl

echo "  Config.pkl generated at /home/tgbot/tg-downloader/config/Config.pkl"

# ---------------------------------------------------------------------------
# Step 12 — Create yt-dlp system config
# (yt-dlp reads /etc/yt-dlp.conf automatically; enables JS challenge solving)
# ---------------------------------------------------------------------------
echo "[12/19] Creating yt-dlp system config at /etc/yt-dlp.conf..."
cat > /etc/yt-dlp.conf << 'EOF'
# System-wide yt-dlp configuration for tg-downloader
--remote-components ejs:github
EOF
echo "  Created /etc/yt-dlp.conf"

# ---------------------------------------------------------------------------
# Step 13 — Optional YouTube cookie setup
# (Datacenter IPs may be blocked by YouTube; browser cookies help bypass this)
# ---------------------------------------------------------------------------
echo
echo "[13/19] Optional: YouTube cookie setup"
echo "  YouTube may block downloads from datacenter IPs."
echo "  Browser cookies from a logged-in session can bypass this."
echo "  Export cookies using a browser extension (e.g. 'Get cookies.txt LOCALLY')"
echo "  and provide the path to the exported file."
echo
read -r -p "Path to cookies.txt (or press Enter to skip): " COOKIES_PATH

if [[ -n "${COOKIES_PATH}" ]]; then
    if [[ ! -f "${COOKIES_PATH}" ]]; then
        echo "  Warning: File not found at ${COOKIES_PATH} — skipping cookie setup" >&2
    else
        COOKIE_DIR="/home/tgbot/.config/yt-dlp"
        sudo -u tgbot mkdir -p "${COOKIE_DIR}"
        cp "${COOKIES_PATH}" "${COOKIE_DIR}/cookies.txt"
        chown tgbot:tgbot "${COOKIE_DIR}/cookies.txt"
        chmod 600 "${COOKIE_DIR}/cookies.txt"

        # Append cookie path to yt-dlp system config
        if ! grep -q "^--cookies" /etc/yt-dlp.conf 2>/dev/null; then
            echo "--cookies /home/tgbot/.config/yt-dlp/cookies.txt" >> /etc/yt-dlp.conf
        fi

        echo "  Cookies installed at ${COOKIE_DIR}/cookies.txt"
        echo "  Added --cookies flag to /etc/yt-dlp.conf"
    fi
else
    echo "  Skipping cookie setup. You can add cookies later by placing a"
    echo "  cookies.txt file at /home/tgbot/.config/yt-dlp/cookies.txt"
    echo "  and adding '--cookies /home/tgbot/.config/yt-dlp/cookies.txt'"
    echo "  to /etc/yt-dlp.conf"
fi

# ---------------------------------------------------------------------------
# Step 14 — Build the bot
# ---------------------------------------------------------------------------
echo "[14/19] Building tg-downloader (make init && make build)..."
sudo -u tgbot bash -c "
    export PATH=/usr/local/go/bin:/usr/local/bin:\$PATH
    export HOME=/home/tgbot
    cd /home/tgbot/tg-downloader
    make init && make build
"
echo "  Binary built at /home/tgbot/tg-downloader/build/tg-downloader"

# ---------------------------------------------------------------------------
# Step 15 — Create systemd service for tg-downloader
# ---------------------------------------------------------------------------
echo "[15/19] Creating systemd service: tg-downloader.service..."
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
# Step 16 — Create systemd service for PO Token server
# (Independent of tg-downloader — bot works even if PO server is down)
# ---------------------------------------------------------------------------
echo "[16/19] Creating systemd service: pot-server.service..."
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
# Step 17 — Enable and start services
# ---------------------------------------------------------------------------
echo "[17/19] Enabling and starting services..."
systemctl daemon-reload
systemctl enable tg-downloader.service pot-server.service
systemctl start tg-downloader.service pot-server.service

# ---------------------------------------------------------------------------
# Step 18 — Install yt-dlp auto-update cron job (weekly Sunday 3:00 AM)
# ---------------------------------------------------------------------------
echo "[18/19] Installing weekly yt-dlp auto-update cron job..."
PIPX_PATH=$(sudo -u tgbot bash -c 'export HOME=/home/tgbot && which pipx')
YTDLP_CRON="0 3 * * 0 ${PIPX_PATH} upgrade yt-dlp"
(crontab -u tgbot -l 2>/dev/null || true; echo "${YTDLP_CRON}") \
    | awk '!x[$0]++' \
    | crontab -u tgbot -
echo "  Cron job installed: ${YTDLP_CRON}"

# ---------------------------------------------------------------------------
# Step 19 — Print summary
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
echo "  Deno:   $(deno --version 2>/dev/null | head -1 || echo 'not found')"
echo "  ffmpeg: $(ffmpeg -version 2>/dev/null | head -1 || echo 'not found')"
echo
echo "Service status:"
systemctl is-active tg-downloader.service && echo "  tg-downloader: running" || echo "  tg-downloader: stopped (check logs)"
systemctl is-active pot-server.service    && echo "  pot-server:    running" || echo "  pot-server:    stopped (check logs)"
echo
echo "yt-dlp system config (/etc/yt-dlp.conf):"
if [[ -f /etc/yt-dlp.conf ]]; then
    sed 's/^/  /' /etc/yt-dlp.conf
else
    echo "  (not found)"
fi
echo
echo "Cron jobs for tgbot user:"
crontab -u tgbot -l 2>/dev/null || echo "  (none)"
echo
echo "View bot logs:         journalctl -u tg-downloader.service -f"
echo "View PO Token logs:    journalctl -u pot-server.service -f"
echo "Update bot:            sudo bash ${SCRIPT_DIR}/install.sh --update"
echo
