# Telegram Media Downloader Bot

A Telegram bot for downloading media from popular social media platforms including Instagram, TikTok, YouTube, Twitter/X, and Reddit. Built with clean architecture principles using Go.

## Features

- **Multi-Platform Support** — Download videos and photos from Instagram, TikTok, YouTube, Twitter/X, Reddit, and anything else yt-dlp supports
- **Album Support** — Batch upload of multi-item posts (photos + videos) with automatic chunking (up to 10 items per album)
- **Format Strategies** — Per-link cascading format fallback (best -> 720p -> worst) configurable in Pkl
- **Concurrent Processing** — Worker pool with configurable worker count, polling interval, and retry logic
- **Group Management** — Activate/deactivate groups for media downloading with admin controls
- **Context-Aware Commands** — Different command sets for direct messages vs group chats
- **Type-Safe Configuration** — Apple Pkl for configuration with compile-time validation and template-based deployment
- **Docker Support** — Multi-stage Docker build with health checks and resource limits
- **One-Command Deployment** — VPS install script (`install.sh`) and local dev setup (`dev.sh`)
- **PO Token Integration** — Automatic YouTube Proof-of-Origin token handling via bgutil plugin

## Architecture

The application follows clean architecture with domain-driven design:

```
src/
├── core/                        # Constants, logger, utilities
│   └── logger/                  # Pluggable logging strategies
├── features/
│   ├── bot/                     # Telegram bot core
│   │   ├── interface/           # BotController (entry point)
│   │   ├── domain/              # Events, services, repository interfaces
│   │   └── data/                # Repository implementations, converters
│   ├── media/                   # Media download & upload pipeline
│   │   ├── domain/              # MediaItem, DownloadResult, strategies
│   │   └── data/                # yt-dlp integration, Telegram upload
│   ├── pottoken/                # YouTube PO Token health checking
│   └── system/                  # Server monitoring (CPU, memory, uptime)
├── dependencies.go              # Uber FX module wiring
└── start.go                     # Application lifecycle hook
```

### Key Patterns

- **Event-Driven Architecture** — Telegram updates converted to typed domain events (sealed interface with 13 event types)
- **Repository Pattern** — Interfaces for all data access (bot, cache, task, media, system)
- **Dependency Injection** — Uber FX for clean composition and lifecycle management
- **Strategy Pattern** — Cascading download format strategies with per-link configuration
- **Worker Pool** — Concurrent media processing with configurable pool size

## Quick Start

### Prerequisites

- **Go 1.25.1** or later
- **Apple Pkl** — Configuration generation (`pkl`, `pkl-gen-go`)
- **yt-dlp** — Media downloading
- **ffmpeg** — Media processing
- **Node.js 20+** — PO Token server (optional, for YouTube)

### Local Development (macOS)

The `dev.sh` script handles everything — installs dependencies via Homebrew, sets up the PO Token server, generates config interactively, builds and runs:

```bash
./dev.sh
```

This will:
1. Install Go, Pkl, ffmpeg, Node.js, yt-dlp via Homebrew/pipx
2. Clone and build the PO Token server
3. Prompt for bot token, admin username, and bot name (first run only)
4. Build the bot and start both services

### Manual Setup

1. **Clone and configure:**
   ```bash
   git clone <repository-url>
   cd tg-downloader
   ```

2. **Create `config/Config.pkl`** from the [Example](config/Example.pkl):
   ```pkl
   amends "templates/TgDownloaderConfig.pkl"

   telegramConfiguration {
       tgBotApiKey = "YOUR_BOT_TOKEN"
       botName = "your_bot_name"
   }

   authConfiguration {
       admininstrators {
           new { userName = "your_username" }
       }
   }

   commandConfiguration {
       commands { ... }
       supportedLinks { ... }
   }

   commonDownloaderConfiguration {
       ytdlpExecutablePath = "/path/to/yt-dlp"
   }
   ```

3. **Build and run:**
   ```bash
   make init    # clean, install deps, generate config & ent code, tidy
   make run     # build and run
   ```

### Development Commands

```bash
make init          # Full setup: clean, install, config-gen, db-gen, tidy
make config-gen    # Regenerate Go code from Pkl config files
make db-gen        # Regenerate Ent ORM code from schemas
make build         # Build binary to build/tg-downloader
make run           # Clean, build, and run
make clean         # Remove build artifacts, generated code, go.sum
make tidy          # go mod tidy
```

## Deployment

### VPS Installation (Ubuntu/Debian)

The `install.sh` script provides a complete 16-step production setup:

```bash
sudo bash install.sh
```

It installs all system dependencies (Go, Pkl, Node.js, yt-dlp, ffmpeg), creates a dedicated `tgbot` user, prompts for configuration, builds the bot, sets up systemd services for both the bot and PO Token server, and configures weekly yt-dlp auto-updates.

**Update an existing installation:**
```bash
sudo bash install.sh --update
```

**Service management:**
```bash
journalctl -u tg-downloader.service -f    # Bot logs
journalctl -u pot-server.service -f        # PO Token server logs
systemctl restart tg-downloader.service    # Restart bot
```

### Docker

```bash
# Build and start
docker compose up -d --build

# View logs
docker compose logs -f

# Stop
docker compose down
```

Mount your own `Config.pkl` with your bot token — the default config uses placeholder values. The container includes yt-dlp and ffmpeg, runs as non-root, and has health checks built in.

Resource defaults: 2 CPU cores / 1 GB memory limit, 0.5 cores / 256 MB reserved.

## Bot Commands

### Group Chat
| Command | Description |
|---------|-------------|
| `/a` | Activate group for downloading |
| `/d` | Deactivate group |
| `/l <url>` | Download media from URL |
| `/i` | Show available commands |

### Direct Message (Admin)
| Command | Description |
|---------|-------------|
| `/a` | List all managed groups |
| `/d <group_id>` | Delete a group |
| `/l` | Show server load info |
| `/i` | Show available commands |

Commands are fully customizable in the Pkl configuration — the short forms above are defaults from `Example.pkl`.

## Supported Platforms

Out of the box, the bot supports:

- **Instagram** — Reels and Posts
- **TikTok** — Short links (`vt.tiktok.com`) and full URLs
- **YouTube** — Regular videos and Shorts
- **Twitter/X** — Via `vxtwitter.com`, `x.com`, or `twitter.com`
- **Reddit** — With optional authentication for restricted content

Each platform has configurable format strategies (e.g., `best`, `best[height<=720]`, `worst`) that cascade on failure. Since the bot uses yt-dlp under the hood, any platform yt-dlp supports can be added by defining a new `supportedLinks` entry in the config.

## PO Token Server (YouTube)

YouTube may require Proof-of-Origin tokens for downloads from certain IPs. The bot integrates with [bgutil-ytdlp-pot-provider](https://github.com/Brainicism/bgutil-ytdlp-pot-provider) to handle this automatically.

- The yt-dlp plugin auto-discovers the server at `http://127.0.0.1:4416`
- The bot performs periodic TCP health checks and logs status changes
- If the PO Token server is unreachable, the bot continues operating — YouTube downloads may fail for some videos, but all other platforms work normally

Both `install.sh` and `dev.sh` set up the PO Token server automatically. For manual setup, see the [bgutil-ytdlp-pot-provider](https://github.com/Brainicism/bgutil-ytdlp-pot-provider) repo.

Configuration (optional — defaults work for standard setup):
```pkl
poTokenConfiguration {
    providerUrl = "http://127.0.0.1:4416"    // null or empty to disable
    healthCheckIntervalSeconds = 900           // 15 minutes
}
```

## Configuration

The bot uses [Apple Pkl](https://pkl-lang.org/) for type-safe configuration. Key configuration sections:

| Section | Purpose |
|---------|---------|
| `telegramConfiguration` | Bot token, name, update polling settings |
| `authConfiguration` | Admin user list |
| `commandConfiguration` | Command definitions, supported link patterns with format strategies |
| `commonDownloaderConfiguration` | yt-dlp path, cookies, IPv4 forcing, sleep intervals, custom headers |
| `videoDownloaderConfiguration` | Output format, max file size, download/upload timeouts |
| `workerConfiguration` | Worker count, polling interval, max retries |
| `poTokenConfiguration` | PO Token server URL and health check interval |

See [Example.pkl](config/Example.pkl) for a complete reference.

## Dependencies

### Core
| Package | Version | Purpose |
|---------|---------|---------|
| [go-telegram-bot-api/v5](https://github.com/go-telegram-bot-api/telegram-bot-api) | v5.5.1 | Telegram Bot API client |
| [entgo.io/ent](https://entgo.io/) | v0.14.5 | ORM and database schema management |
| [go.uber.org/fx](https://uber-go.github.io/fx/) | v1.24.0 | Dependency injection framework |
| [apple/pkl-go](https://github.com/apple/pkl-go) | v0.12.1 | Type-safe configuration management |
| [mattn/go-sqlite3](https://github.com/mattn/go-sqlite3) | v1.14.17 | SQLite database driver |

### Media Processing
| Package | Version | Purpose |
|---------|---------|---------|
| [lrstanley/go-ytdlp](https://github.com/lrstanley/go-ytdlp) | v1.2.4 | Go wrapper for yt-dlp |

### System Monitoring
| Package | Version | Purpose |
|---------|---------|---------|
| [shirou/gopsutil/v3](https://github.com/shirou/gopsutil) | v3.24.5 | System and process utilities |

## License

This code is licensed under the MIT license.

Even if basically the only thing this bot does is automatically downloading media from links, so there's not that big of a risk, I don't claim any responsibilities at all for the usage of this bot and the eventual consequences.
