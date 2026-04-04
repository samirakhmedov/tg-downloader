package repository

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"sync/atomic"
	"time"

	"tg-downloader/env"
	"tg-downloader/src/core/logger"

	"go.uber.org/fx"
)

// POTokenChecker performs periodic TCP reachability checks against the
// bgutil-ytdlp-pot-provider server and tracks state transitions for logging.
type POTokenChecker struct {
	reachable   atomic.Bool
	providerURL string
	interval    time.Duration
	logger      *logger.Logger
	cancel      context.CancelFunc
}

// NewPOTokenChecker creates a POTokenChecker and registers FX lifecycle hooks.
// If providerUrl is nil or empty, no hooks are registered and IsReachable() always returns false.
func NewPOTokenChecker(cfg env.TGDownloader, lc fx.Lifecycle, log *logger.Logger) *POTokenChecker {
	c := &POTokenChecker{
		logger: log,
	}

	// Nil-check the *string field before dereferencing.
	if cfg.PoTokenConfiguration.ProviderUrl == nil || *cfg.PoTokenConfiguration.ProviderUrl == "" {
		return c
	}

	c.providerURL = *cfg.PoTokenConfiguration.ProviderUrl
	c.interval = time.Duration(cfg.PoTokenConfiguration.HealthCheckIntervalSeconds) * time.Second

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			// Perform initial check synchronously (non-fatal — always return nil).
			c.checkAndLog(true)

			// Use a fresh background context for the goroutine — the OnStart ctx is short-lived.
			bgCtx, cancel := context.WithCancel(context.Background())
			c.cancel = cancel
			go c.runPeriodicCheck(bgCtx)

			return nil
		},
		OnStop: func(ctx context.Context) error {
			if c.cancel != nil {
				c.cancel()
			}
			return nil
		},
	})

	return c
}

// NewPOTokenCheckerForTest creates a POTokenChecker without FX lifecycle hooks.
// This constructor is exported for use in unit tests only.
func NewPOTokenCheckerForTest(providerURL string, intervalSeconds int, log *logger.Logger) *POTokenChecker {
	return &POTokenChecker{
		providerURL: providerURL,
		interval:    time.Duration(intervalSeconds) * time.Second,
		logger:      log,
	}
}

// IsReachable returns true if the last health check found the server reachable.
func (c *POTokenChecker) IsReachable() bool {
	return c.reachable.Load()
}

// ProviderURL returns the configured PO Token server URL.
// Returns empty string if PO Token checking is disabled.
func (c *POTokenChecker) ProviderURL() string {
	return c.providerURL
}

// SetReachable sets the internal reachable state directly.
// Exported for use in unit tests to pre-condition state.
func (c *POTokenChecker) SetReachable(v bool) {
	c.reachable.Store(v)
}

// CheckNow performs a single health check and logs according to isStartup semantics.
// Exported for use in unit tests.
func (c *POTokenChecker) CheckNow(isStartup bool) {
	c.checkAndLog(isStartup)
}

// checkAndLog performs a TCP check and logs on startup (always) or state transitions (periodic).
func (c *POTokenChecker) checkAndLog(isStartup bool) {
	now := TcpReachable(c.providerURL, 2*time.Second)
	prev := c.reachable.Swap(now)

	if isStartup {
		if now {
			c.logger.Info(fmt.Sprintf("PO Token server: online (%s)", c.providerURL))
		} else {
			c.logger.Warn(fmt.Sprintf("PO Token server: unreachable (%s)", c.providerURL))
		}
		return
	}

	// Periodic: log only on state transitions.
	if !prev && now {
		c.logger.Info(fmt.Sprintf("PO Token server: back online (%s)", c.providerURL))
	} else if prev && !now {
		c.logger.Warn(fmt.Sprintf("PO Token server: went unreachable (%s)", c.providerURL))
	}
}

// runPeriodicCheck loops on the ticker until the context is cancelled.
func (c *POTokenChecker) runPeriodicCheck(ctx context.Context) {
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.checkAndLog(false)
		case <-ctx.Done():
			return
		}
	}
}

// TcpReachable returns true if a TCP connection can be established to the host:port
// extracted from rawURL within the given timeout.
// Exported so unit tests can call it directly.
func TcpReachable(rawURL string, timeout time.Duration) bool {
	u, err := url.Parse(rawURL)
	if err != nil || u.Host == "" {
		return false
	}
	conn, err := net.DialTimeout("tcp", u.Host, timeout)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}
