package strategy

import "github.com/lrstanley/go-ytdlp"

// DownloadStrategy implements IDownloadStrategy for any platform.
// Applies the format string and optional platform-specific extractor args.
type DownloadStrategy struct {
	name          string
	format        string
	extractorArgs []string
}

// NewDownloadStrategy creates a strategy that applies format and optional extractor args.
func NewDownloadStrategy(name, format string, extractorArgs []string) IDownloadStrategy {
	return &DownloadStrategy{
		name:          name,
		format:        format,
		extractorArgs: extractorArgs,
	}
}

func (s *DownloadStrategy) Name() string { return s.name }

func (s *DownloadStrategy) Apply(cmd *ytdlp.Command) *ytdlp.Command {
	cmd = cmd.Format(s.format)
	for _, arg := range s.extractorArgs {
		if arg != "" {
			cmd = cmd.ExtractorArgs(arg)
		}
	}
	return cmd
}
