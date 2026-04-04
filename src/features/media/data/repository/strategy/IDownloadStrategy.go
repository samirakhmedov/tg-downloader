package strategy

import "github.com/lrstanley/go-ytdlp"

// IDownloadStrategy configures a ytdlp.Command for one quality attempt.
// Each implementation sets the format string and any platform-specific options.
type IDownloadStrategy interface {
	// Name returns a human-readable label used in log messages (e.g., "720p mp4").
	Name() string

	// Apply configures the provided command with this strategy's format and options.
	// Returns the modified command to support method chaining.
	Apply(cmd *ytdlp.Command) *ytdlp.Command
}
