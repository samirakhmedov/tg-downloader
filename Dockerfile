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

# Install Pkl CLI (must match version used in development)
ARG PKL_VERSION=0.31.1
RUN ARCH=$(uname -m) && \
    PKL_ARCH=$(if [ "$ARCH" = "aarch64" ]; then echo "aarch64"; else echo "amd64"; fi) && \
    curl -L -o /usr/local/bin/pkl \
    "https://github.com/apple/pkl/releases/download/${PKL_VERSION}/pkl-linux-${PKL_ARCH}" \
    && chmod +x /usr/local/bin/pkl

# Set working directory
WORKDIR /build

# Copy go module files first for better caching
COPY go.mod go.sum* ./

# Download dependencies (includes pkl-gen-go via tool directive in go.mod)
RUN go mod download

# Copy config files needed for code generation
COPY config/ ./config/

# Generate Go code from Pkl configuration template
# (uses go tool directive — no separate install needed)
RUN go tool pkl-gen-go config/templates/TgDownloaderConfig.pkl

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
    curl \
    unzip \
    python3 \
    python3-pip \
    ffmpeg \
    && rm -rf /var/lib/apt/lists/*

# Install Deno (required by yt-dlp for YouTube JS challenge solving)
RUN curl -fsSL https://deno.land/install.sh | DENO_INSTALL=/usr/local sh

# Install yt-dlp + bgutil PO Token plugin
RUN pip3 install --break-system-packages --no-cache-dir \
    yt-dlp \
    bgutil-ytdlp-pot-provider

# Create yt-dlp system config (enables remote JS components for YouTube)
RUN echo '--remote-components ejs:github' > /etc/yt-dlp.conf

# Create app user for security
RUN useradd -m -s /bin/bash appuser

# Set working directory
WORKDIR /app

# Create necessary directories
RUN mkdir -p /app/config/templates /app/output/videos /app/temp/videos /app/data \
    && chown -R appuser:appuser /app

# Copy the binary and pkl CLI from builder stage
# (pkl is needed at runtime for Config.pkl evaluation via pkl-go)
COPY --from=builder /build/tg-downloader /app/tg-downloader
COPY --from=builder /usr/local/bin/pkl /usr/local/bin/pkl

# Copy config templates (required for Pkl amends at runtime)
COPY --chown=appuser:appuser config/templates/ /app/config/templates/

# Switch to non-root user
USER appuser

# Expose volumes for configuration and data persistence
VOLUME ["/app/config", "/app/data", "/app/output"]

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD pgrep -x tg-downloader || exit 1

# Run the application
ENTRYPOINT ["/app/tg-downloader"]
