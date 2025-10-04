package repository

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"tg-downloader/env"
	"tg-downloader/src/core"
	"tg-downloader/src/features/video/domain/entity"
	"tg-downloader/src/features/video/domain/repository"

	"github.com/lrstanley/go-ytdlp"
)

type VideoDownloadRepository struct {
	environment env.TGDownloader
}

func NewVideoDownloadRepository(environment env.TGDownloader) repository.IVideoDownloadRepository {
	return &VideoDownloadRepository{
		environment: environment,
	}
}

func (r *VideoDownloadRepository) ValidateURL(url string) (bool, string, error) {
	// First check if it's a valid URL format
	urlRegex := regexp.MustCompile(core.URLRegexPattern)
	if !urlRegex.MatchString(url) {
		return false, "", fmt.Errorf("invalid URL format")
	}

	// Check against supported patterns
	for _, linkPattern := range r.environment.CommandConfiguration.SupportedLinks {
		matched, err := regexp.MatchString(linkPattern.Pattern, url)
		if err != nil {
			continue
		}
		if matched {
			return true, linkPattern.Name, nil
		}
	}

	// Build error message with supported formats
	var supportedFormats string
	for i, linkPattern := range r.environment.CommandConfiguration.SupportedLinks {
		if i > 0 {
			supportedFormats += "\n"
		}
		supportedFormats += fmt.Sprintf("• %s: %s", linkPattern.Name, linkPattern.Example)
	}

	return false, "", fmt.Errorf("unsupported video format. Supported formats:\n%s", supportedFormats)
}

func (r *VideoDownloadRepository) DownloadVideo(url string, outputDir string) (*entity.VideoProcessResult, error) {
	// Ensure output directory exists
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return &entity.VideoProcessResult{
			Success: false,
			Error:   fmt.Errorf("failed to create output directory: %w", err),
		}, err
	}

	// Configure yt-dlp options with configured executable path
	dl := ytdlp.New().
		SetExecutable(r.environment.CommonDownloaderConfiguration.YtdlpExecutablePath).
		Format(r.environment.VideoDownloaderConfiguration.VideoQuality).
		Output(filepath.Join(outputDir, "%(title)s.%(ext)s")).
		NoCheckCertificates()

	// Add format specification if needed
	if r.environment.VideoDownloaderConfiguration.OutputFormat != "" {
		dl = dl.RecodeVideo(r.environment.VideoDownloaderConfiguration.OutputFormat)
	}

	// Apply yt-dlp configuration options
	dl = r.applyYtdlpOptions(dl)

	// Execute download
	_, err := dl.Run(context.Background(), url)
	if err != nil {
		return &entity.VideoProcessResult{
			Success: false,
			Error:   fmt.Errorf("download failed: %w", err),
		}, err
	}

	// Find the downloaded file by scanning the output directory
	var downloadedFile string
	var fileSize int64

	entries, err := os.ReadDir(outputDir)
	if err != nil {
		return &entity.VideoProcessResult{
			Success: false,
			Error:   fmt.Errorf("failed to read output directory: %w", err),
		}, err
	}

	// Find the most recently created file
	var latestFile os.FileInfo
	var latestPath string
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if latestFile == nil || info.ModTime().After(latestFile.ModTime()) {
			latestFile = info
			latestPath = filepath.Join(outputDir, entry.Name())
		}
	}

	if latestPath != "" {
		downloadedFile = latestPath
		fileSize = latestFile.Size()
	}

	if downloadedFile == "" {
		return &entity.VideoProcessResult{
			Success: false,
			Error:   fmt.Errorf("no file was downloaded"),
		}, fmt.Errorf("download result empty")
	}

	// Check file size limit
	maxSizeMB := int64(r.environment.VideoDownloaderConfiguration.MaxFileSizeMB)
	if fileSize > maxSizeMB*1024*1024 {
		// Clean up oversized file
		os.Remove(downloadedFile)
		return &entity.VideoProcessResult{
			Success: false,
			Error:   fmt.Errorf("file size %d MB exceeds limit of %d MB", fileSize/(1024*1024), maxSizeMB),
		}, fmt.Errorf("file too large")
	}

	return &entity.VideoProcessResult{
		Success:  true,
		FilePath: downloadedFile,
		FileName: filepath.Base(downloadedFile),
		FileSize: fileSize,
	}, nil
}

// applyYtdlpOptions applies yt-dlp configuration options from the environment
func (r *VideoDownloadRepository) applyYtdlpOptions(dl *ytdlp.Command) *ytdlp.Command {
	config := r.environment.CommonDownloaderConfiguration

	// Browser cookies for authentication
	if config.CookiesFromBrowser != nil && *config.CookiesFromBrowser != "" {
		dl = dl.CookiesFromBrowser(*config.CookiesFromBrowser)
	}

	// Force IPv4 if configured
	if config.ForceIPv4 {
		dl = dl.ForceIPv4()
	}

	// Sleep interval for rate limiting
	if config.SleepInterval > 0 {
		dl = dl.SleepInterval(config.SleepInterval)
	}

	// Maximum sleep interval for random delays
	if config.MaxSleepInterval > 0 {
		dl = dl.MaxSleepInterval(config.MaxSleepInterval)
	}

	// Custom User-Agent via headers (UserAgent method is deprecated)
	if config.UserAgent != nil && *config.UserAgent != "" {
		dl = dl.AddHeaders("User-Agent:" + *config.UserAgent)
	}

	// TikTok-specific extractor arguments
	if config.TiktokApiHostname != nil && *config.TiktokApiHostname != "" {
		dl = dl.ExtractorArgs("tiktok:api_hostname=" + *config.TiktokApiHostname)
	}

	// Additional extractor arguments
	for _, extractorArg := range config.ExtractorArgs {
		if extractorArg != "" {
			dl = dl.ExtractorArgs(extractorArg)
		}
	}

	// Custom HTTP headers
	for _, header := range config.CustomHeaders {
		if header != "" {
			dl = dl.AddHeaders(header)
		}
	}

	return dl
}
