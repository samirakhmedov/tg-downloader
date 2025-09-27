# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Development Commands

### Setup and Configuration
```bash
make init              # Complete setup: clean, install, config-gen, go mod tidy
make config-gen        # Generate Go code from Pkl config files
make install           # Download Go dependencies
```

### Build and Run
```bash
make build             # Build binary to build/tg-downloader
make run               # Clean, build, and run the application
make clean             # Remove build artifacts and generated files
```

### Direct Go Commands
```bash
go build -o build/tg-downloader .  # Build binary
go mod tidy                        # Clean up dependencies
```

## Architecture Overview

This is a Telegram bot application for file downloading with clean architecture principles:

### Core Technology Stack
- **Go 1.25.1** with Telegram Bot API (`go-telegram-bot-api/v5`)
- **Dependency Injection** using Uber FX framework
- **Database** SQLite with Ent ORM (in-memory by default)
- **Configuration** Apple Pkl for type-safe config generation

### Project Structure
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
│       ├── repository/      # Repository implementations
│       └── converter/       # Data conversion logic
├── dependencies.go          # FX dependency injection setup
└── start.go                 # Application startup logic

config/
├── Config.pkl               # Main configuration file
└── templates/               # Pkl configuration templates

ent/                         # Ent ORM generated code
env/                         # Generated Go code from Pkl configs
```

### Dependency Injection Flow
The application uses Uber FX for dependency injection in `main.go`:
1. **NewBotConfiguration** → Loads Pkl config from `config/Config.pkl`
2. **NewBotAPI** → Creates Telegram Bot API client
3. **NewDatabase** → Sets up SQLite + Ent ORM with migrations
4. **NewBotRepository** → Creates main bot repository (Telegram API interactions)
5. **NewBotCacheRepository** → Creates cache repository (database operations)
6. **NewBotService** → Business logic service with command filtering
7. **NewBotController** → Event processing controller
8. **StartBot** → Lifecycle management and event processing startup

### Event-Driven Architecture
The bot operates on an event-driven model:
- **BotEvents** channel receives Telegram updates
- **UpdateToBotEventConverter** transforms Telegram updates to domain events
- **BotController** processes events with command updates and business logic separation
- **BotService** handles business logic with admin validation and context-aware messaging

### Key Domain Concepts

#### Command System
- **Context-aware commands**: Different command sets for direct messages vs group chats
- **Access levels**: User vs Admin permissions (defined in `env/accesslevel`)
- **Dynamic command updates**: Commands are updated per-user to avoid conflicts between admin/user commands in the same group
- **Scoped commands**:
  - Direct: `/a` (getAllGroups), `/l` (getServerLoad), `/d` (deleteGroup), `/i` (getBotCommands)
  - Group: `/a` (activateGroup), `/d` (deactivateGroup), `/l` (loadResource), `/i` (getBotCommands)

#### Group Management
- **Activation/Deactivation**: Admins can activate/deactivate groups within group chats
- **Deletion**: Admins can delete groups via private chat with bot
- **Listing**: Admins can list all managed groups via private chat

#### Repository Pattern
- **IBotRepository**: Telegram API operations (messaging, admin checks, command setting)
- **IBotCacheRepository**: Database operations for group management
- Both use int64 for IDs to match Telegram's ID format

### Configuration Management
The application uses Apple Pkl for type-safe configuration:
- **config/Config.pkl** defines bot settings, commands, and admin users
- **make config-gen** regenerates Go code from Pkl files
- Generated code appears in `env/` directory
- Configuration includes command mappings, access levels, and bot API settings

### Error Handling Pattern
- **ErrorDirect/ErrorGroup events** for structured error messaging
- **Context-aware error handling**: Errors sent to appropriate chat (direct vs group)
- Service methods return errors that are converted to user-friendly messages

### Testing and Development
- Database runs in-memory mode by default (`DatabaseSource` in core/Constants.go)
- Debug mode can be enabled via Pkl configuration
- Bot gracefully handles shutdown signals (SIGTERM, SIGINT)