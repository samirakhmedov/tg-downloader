# Telegram Video Downloader Bot

A robust Telegram bot for downloading videos from popular social media platforms including Instagram, TikTok, YouTube Shorts, and Twitter/X. Built with clean architecture principles using Go and modern development practices.

## ✨ Features

- **Multi-Platform Support**: Download videos from Instagram Reels, TikTok, YouTube Shorts, and Twitter/X
- **Group Management**: Activate/deactivate groups for video downloading
- **Admin Controls**: Comprehensive admin panel with group management and server monitoring
- **Context-Aware Commands**: Different command sets for direct messages vs group chats
- **Concurrent Processing**: Multi-worker video processing with configurable worker pools
- **Type-Safe Configuration**: Apple Pkl for configuration management with compile-time validation
- **Clean Architecture**: Domain-driven design with dependency injection using Uber FX

## 🏗️ Architecture Overview

The application follows clean architecture principles with clear separation of concerns:

```
src/
├── core/                    # Constants and core utilities
├── features/bot/            # Bot feature implementation
│   ├── interface/           # Controllers (entry points)
│   ├── domain/              # Business logic layer
│   │   ├── entity/          # Domain entities and events
│   │   ├── service/         # Business logic services
│   │   └── repository/      # Repository interfaces
│   └── data/                # Data layer implementation
├── features/system/         # System monitoring features
├── features/video/          # Video processing features
└── dependencies.go          # FX dependency injection setup
```

### Key Architectural Patterns

- **Event-Driven Architecture**: Telegram updates converted to domain events
- **Repository Pattern**: Abstracted data access with interfaces
- **Dependency Injection**: Uber FX for clean dependency management
- **Command Pattern**: Context-aware command processing
- **Worker Pool Pattern**: Concurrent video processing

## 🚀 Quick Start

### Prerequisites

- **Go 1.25.1** or later
- **Apple Pkl**: For configuration generation (`pkl`, `pkl-go`)
- **yt-dlp**: Video downloading utility
- **SQLite**: Database (automatically managed)

### Installation

1. **Clone the repository**:
   ```bash
   git clone <repository-url>
   cd tg-downloader
   ```

2. **Configure the bot**:
   Create `config/Config.pkl` with your bot token and settings:
   ```pkl
   telegramConfiguration {
       tgBotApiKey = "YOUR_BOT_TOKEN_HERE"
       updateTimeout = 0
       updateOffset = 0
       updateLimit = 2
       botName = "your_bot_name"
   }

   authConfiguration { ... }

   commandConfiguration { ... }

   commonDownloaderConfiguration { ... }

   videoDownloaderConfiguration { ... }

   workerConfiguration { ... }

   debug = false
   ```

   For a reference, look at the [example](/config/Example.pkl).

3. **Install dependencies and setup**:
   ```bash
   make init
   ```

4. **Build and run**:
   ```bash
   make run
   ```

### Development Commands

```bash
# Complete setup
make init              # Clean, install, config-gen, go mod tidy

# Configuration
make config-gen        # Generate Go code from Pkl config files

# Build and run
make build             # Build binary to build/tg-downloader
make run               # Clean, build, and run the application
make clean             # Remove build artifacts
```

## 🤖 Bot Commands

### Group Chat Commands
- `/a` - Activate group for downloading
- `/d` - Deactivate group
- `/l <url>` - Download video from URL
- `/i` - Get bot commands

### Direct Message Commands (Admin)
- `/a` - Get all groups
- `/d <group_id>` - Delete group
- `/l` - Get server load information
- `/i` - Get bot commands

You could customize commands at configuration, but make sure to add support in code.

## 🔑 PO Token Server (YouTube)

YouTube may require Proof-of-Origin (PO) tokens for downloads from certain IP addresses. The bot integrates with [bgutil-ytdlp-pot-provider](https://github.com/Brainicism/bgutil-ytdlp-pot-provider) to handle this automatically.

### How It Works

- The `bgutil-ytdlp-pot-provider` yt-dlp plugin auto-discovers its server at `http://127.0.0.1:4416` when installed
- No changes to download configuration are needed — the plugin handles PO tokens transparently
- The bot performs periodic TCP health checks and logs status changes
- If the PO Token server is unreachable, the bot continues operating — YouTube downloads may fail for some videos, but all other platforms work normally

### Setup

1. **Install the yt-dlp plugin:**
   ```bash
   pip install bgutil-ytdlp-pot-provider
   ```

2. **Clone and build the token server:**
   ```bash
   git clone https://github.com/Brainicism/bgutil-ytdlp-pot-provider.git
   cd bgutil-ytdlp-pot-provider/server
   npm install
   npm run build
   ```

3. **Run the token server:**
   ```bash
   npm start
   ```
   The server listens on port 4416 by default.

4. **Configure the bot** (optional — defaults work for standard setup):
   In `config/Config.pkl`:
   ```pkl
   poTokenConfiguration {
       providerUrl = "http://127.0.0.1:4416"    // null or empty to disable
       healthCheckIntervalSeconds = 900           // 15 minutes
   }
   ```

### Monitoring

- **Startup:** The bot logs PO Token server status at startup (Info if online, Warn if unreachable)
- **Periodic:** Status changes are logged automatically (every 15 minutes by default)
- **On demand:** Use the `/l` command (admin only) to see current PO Token server status

## 🔧 Third-Party Dependencies

### Core Dependencies

| Package | Version | Purpose |
|---------|---------|---------|
| [go-telegram-bot-api/v5](https://github.com/go-telegram-bot-api/telegram-bot-api) | v5.5.1 | Telegram Bot API client |
| [entgo.io/ent](https://entgo.io/) | v0.14.5 | ORM and database schema management |
| [go.uber.org/fx](https://uber-go.github.io/fx/) | v1.24.0 | Dependency injection framework |
| [apple/pkl-go](https://github.com/apple/pkl-go) | v0.11.1 | Type-safe configuration management |
| [mattn/go-sqlite3](https://github.com/mattn/go-sqlite3) | v1.14.17 | SQLite database driver |

### Video Processing
| Package | Version | Purpose |
|---------|---------|---------|
| [lrstanley/go-ytdlp](https://github.com/lrstanley/go-ytdlp) | v1.2.4 | Go wrapper for yt-dlp video downloader |

### System Monitoring
| Package | Version | Purpose |
|---------|---------|---------|
| [shirou/gopsutil/v3](https://github.com/shirou/gopsutil) | v3.24.5 | System and process utilities |

## 🎯 Supported Platforms

- **Instagram Reels**: `https://www.instagram.com/reel/*`
- **TikTok**: `https://vt.tiktok.com/*`
- **YouTube Shorts**: `https://youtube.com/shorts/*`
- **Twitter/X**: `https://vxtwitter.com/*/status/*`
- Basically anything yt-dlp currently supports, check the configuration

## ✅ Pros

### Architecture Benefits
- **Clean Architecture**: Clear separation of concerns with domain-driven design
- **Type Safety**: Pkl configuration prevents runtime configuration errors
- **Dependency Injection**: Testable and maintainable code with Uber FX
- **Event-Driven**: Scalable event processing architecture
- **Repository Pattern**: Abstracted data access for easy testing and swapping

### Technical Strengths
- **Concurrent Processing**: Multi-worker video processing for high throughput
- **Context-Aware Commands**: Smart command handling based on chat type
- **Robust Error Handling**: Comprehensive error handling with user-friendly messages
- **Database Integration**: Automatic schema management with Ent ORM
- **Graceful Shutdown**: Proper resource cleanup and shutdown handling

### Operational Benefits
- **Configuration Management**: Type-safe, validated configuration
- **Admin Controls**: Comprehensive admin panel for monitoring and management
- **Group Management**: Fine-grained control over group activation/deactivation
- **Monitoring**: Built-in system monitoring capabilities

## ⚠️ Cons

### Complexity Overhead
- **Learning Curve**: Multiple frameworks (FX, Ent, Pkl) require domain knowledge
- **Configuration Complexity**: Pkl adds build-time dependency and complexity
- **Architecture Overhead**: Clean architecture may be overkill for simpler use cases

### Technical Limitations
- **Single Bot Instance**: No horizontal scaling support out of the box
- **File System Storage**: Temporary video storage on local filesystem
- **External Dependencies**: Relies on yt-dlp binary availability
- **Platform Limitations**: Dependent on yt-dlp platform support updates

### Operational Considerations
- **Resource Usage**: Video processing can be memory and CPU intensive
- **Storage Management**: No automatic cleanup of failed downloads
- **Error Recovery**: Limited retry mechanisms for failed video processing
- **Monitoring**: Basic logging without structured observability

## 🔒 Security Considerations

- Bot token stored in configuration files (consider environment variables)
- No rate limiting implemented
- File system access for video processing
- Database file permissions should be secured

## 📝 Development

### Adding New Video Platforms
1. Update `supportedLinks` in `config/Config.pkl`
2. Ensure yt-dlp supports the platform
3. Test URL validation patterns

### Extending Commands
1. Add command definition in `config/Config.pkl`
2. Create corresponding event in `BotEvents.go`
3. Implement handler in `BotService.go`
4. Update converter in `UpdateToBotEventConverter.go`

## 📄 License

This code is licensed under the MIT license.

Even if basically the only thing this bot does is automatically downloading videos from links, so there's not that big of a risk, I don't claim any responsibilties at all for the usage of this bot and the eventual consequences.