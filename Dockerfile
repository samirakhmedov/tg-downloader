# =============================================================================
# Build Stage
# =============================================================================
FROM golang:1.25-bookworm AS builder

# Install build dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
    curl \
    ca-certificates \
    gcc \
    libc6-dev \
    && rm -rf /var/lib/apt/lists/*

# Install Pkl CLI
ARG PKL_VERSION=0.28.0
RUN curl -L -o /tmp/pkl.jar "https://github.com/apple/pkl/releases/download/${PKL_VERSION}/pkl-linux-amd64" \
    && chmod +x /tmp/pkl.jar \
    && mv /tmp/pkl.jar /usr/local/bin/pkl

# Set working directory
WORKDIR /build

# Copy go module files first for better caching
COPY go.mod go.sum* ./

# Download dependencies
RUN go mod download

# Install pkl-gen-go tool
RUN go install github.com/apple/pkl-go/cmd/pkl-gen-go@latest

# Copy config files needed for code generation
COPY config/ ./config/

# Generate Go code from Pkl configuration
RUN pkl eval config/Config.pkl && \
    pkl-gen-go config/Config.pkl

# Copy the rest of the source code
COPY . .

# Generate ent code
RUN go generate ./ent

# Tidy modules after code generation
RUN go mod tidy

# Build the application with CGO enabled (required for SQLite3)
RUN CGO_ENABLED=1 GOOS=linux go build -o /build/tg-downloader -ldflags="-s -w" .

# =============================================================================
# Runtime Stage
# =============================================================================
FROM ubuntu:24.04

# Prevent interactive prompts during package installation
ENV DEBIAN_FRONTEND=noninteractive

# Install runtime dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    python3 \
    python3-pip \
    ffmpeg \
    && rm -rf /var/lib/apt/lists/*

# Install yt-dlp via pip
RUN pip3 install --break-system-packages --no-cache-dir yt-dlp

# Create app user for security
RUN useradd -m -s /bin/bash appuser

# Set working directory
WORKDIR /app

# Create necessary directories
RUN mkdir -p /app/config/templates /app/output/videos /app/temp/videos /app/data \
    && chown -R appuser:appuser /app

# Copy the binary from builder stage
COPY --from=builder /build/tg-downloader /app/tg-downloader

# Copy config templates and default config
COPY --chown=appuser:appuser config/templates/ /app/config/templates/
COPY --chown=appuser:appuser config/Example.pkl /app/config/Config.pkl

# Update the config to use the correct yt-dlp path for Linux
# Note: Users should mount their own config with proper bot token
RUN sed -i 's|ytdlpExecutablePath = ".*"|ytdlpExecutablePath = "/usr/local/bin/yt-dlp"|' /app/config/Config.pkl

# Switch to non-root user
USER appuser

# Expose volumes for configuration and data persistence
VOLUME ["/app/config", "/app/data", "/app/output"]

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD pgrep -x tg-downloader || exit 1

# Run the application
ENTRYPOINT ["/app/tg-downloader"]
