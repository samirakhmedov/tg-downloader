package entity

import "tg-downloader/env"

// PlatformInfo holds the detected platform identity and download configuration for a URL.
// Constructed by MediaDownloadRepository.ValidateURL() and passed to DownloadMedia().
type PlatformInfo struct {
	// Name is the human-readable platform name from the matched SupportedLinks entry
	// (e.g., "YouTube Shorts", "TikTok", "Instagram Reels").
	Name string

	// Strategies is the ordered list of format strategies from the matched pattern.
	Strategies []env.FormatStrategy

	// ExtractorArgs is the list of platform-specific extractor arguments from the matched pattern.
	ExtractorArgs []string

	// Username for site authentication (optional, per-platform).
	Username *string

	// Password for site authentication (optional, per-platform).
	Password *string
}
