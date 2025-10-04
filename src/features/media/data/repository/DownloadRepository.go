package repository

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"tg-downloader/env"
	"tg-downloader/src/core"
	"tg-downloader/src/features/media/domain/entity"
	"tg-downloader/src/features/media/domain/repository"

	"github.com/google/uuid"
	"github.com/lrstanley/go-ytdlp"
)

type DownloadRepository struct {
	environment env.TGDownloader
}

func NewDownloadRepository(environment env.TGDownloader) repository.IDownloadRepository {
	return &DownloadRepository{
		environment: environment,
	}
}

func (r *DownloadRepository) ValidateURL(url string) (bool, string, error) {
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

	return false, "", fmt.Errorf("unsupported media format. Supported formats:\n%s", supportedFormats)
}

func (r *DownloadRepository) Download(url string, outputDir string) (*entity.MediaProcessResult, error) {
	// Ensure output directory exists
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return &entity.MediaProcessResult{
			Success: false,
			Error:   fmt.Errorf("failed to create output directory: %w", err),
		}, err
	}

	// Generate UUID for this download session
	sessionID := uuid.New().String()

	// First, try to download as video/mixed media
	result, err := r.downloadMedia(url, outputDir, sessionID)
	if err != nil {
		return result, err
	}

	return result, nil
}

func (r *DownloadRepository) downloadMedia(url string, outputDir string, sessionID string) (*entity.MediaProcessResult, error) {
	// Configure yt-dlp with UUID-based output template
	outputTemplate := filepath.Join(outputDir, fmt.Sprintf("%s-%%(autonumber)s.%%(ext)s", sessionID))

	// First attempt: try downloading with default settings (works for videos and most content)
	dl := ytdlp.New().
		SetExecutable(r.environment.CommonDownloaderConfiguration.YtdlpExecutablePath).
		Output(outputTemplate).
		NoCheckCertificates().
		Format(r.environment.VideoDownloaderConfiguration.OutputFormat)

	// Apply yt-dlp configuration options
	dl = r.applyYtdlpOptions(dl)

	// Execute download
	dl.Run(context.Background(), url)

	// If error occurs, it might be image-only content (like TikTok carousels)
	// Don't return immediately, check if files were downloaded first

	// Find all downloaded files by scanning the output directory
	entries, err := os.ReadDir(outputDir)
	if err != nil {
		return &entity.MediaProcessResult{
			Success: false,
			Error:   fmt.Errorf("failed to read output directory: %w", err),
		}, err
	}

	// Collect files from this session
	var mediaFiles []entity.MediaFile
	var totalSize int64

	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name(), sessionID) {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		filePath := filepath.Join(outputDir, entry.Name())
		mediaType := r.detectMediaType(entry.Name())

		mediaFiles = append(mediaFiles, entity.MediaFile{
			FilePath:  filePath,
			FileName:  entry.Name(),
			FileSize:  info.Size(),
			MediaType: mediaType,
		})

		totalSize += info.Size()
	}

	// If no files downloaded, try the fallback method for images and audio
	if len(mediaFiles) == 0 {
		return r.downloadImagesAndAudio(url, outputDir, sessionID)
	}

	// Check total file size limit (using video config for now)
	maxSizeMB := int64(r.environment.VideoDownloaderConfiguration.MaxFileSizeMB)
	if totalSize > maxSizeMB*1024*1024 {
		// Clean up oversized files
		for _, file := range mediaFiles {
			os.Remove(file.FilePath)
		}
		return &entity.MediaProcessResult{
			Success: false,
			Error:   fmt.Errorf("total file size %d MB exceeds limit of %d MB", totalSize/(1024*1024), maxSizeMB),
		}, fmt.Errorf("files too large")
	}

	return &entity.MediaProcessResult{
		Success: true,
		Files:   mediaFiles,
	}, nil
}

func (r *DownloadRepository) downloadImagesAndAudio(url string, outputDir string, sessionID string) (*entity.MediaProcessResult, error) {
	var mediaFiles []entity.MediaFile

	// For TikTok image carousels and similar, we need to:
	// 1. Download all media items (images)
	// 2. Download audio separately if available

	// Try downloading all media items with specific format for images
	mediaTemplate := filepath.Join(outputDir, fmt.Sprintf("%s-%%(autonumber)s.%%(ext)s", sessionID))
	dlMedia := ytdlp.New().
		SetExecutable(r.environment.CommonDownloaderConfiguration.YtdlpExecutablePath).
		Output(mediaTemplate).
		NoCheckCertificates().
		WriteAllThumbnails().                                           // This helps with TikTok image posts
		Format(r.environment.PhotoDownloaderConfiguration.OutputFormat) // Download all available formats

	dlMedia = r.applyYtdlpOptions(dlMedia)
	dlMedia.Run(context.Background(), url) // Ignore errors, check files later

	// Try downloading audio separately
	audioTemplate := filepath.Join(outputDir, fmt.Sprintf("%s-audio.%%(ext)s", sessionID))
	dlAudio := ytdlp.New().
		SetExecutable(r.environment.CommonDownloaderConfiguration.YtdlpExecutablePath).
		Output(audioTemplate).
		NoCheckCertificates().
		Format(r.environment.AudioDownloaderConfiguration.OutputFormat).
		ExtractAudio()

	dlAudio = r.applyYtdlpOptions(dlAudio)
	dlAudio.Run(context.Background(), url) // Ignore errors, check files later

	// Collect all downloaded files
	entries, err := os.ReadDir(outputDir)
	if err != nil {
		return &entity.MediaProcessResult{
			Success: false,
			Error:   fmt.Errorf("failed to read output directory: %w", err),
		}, err
	}

	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}

		filePath := filepath.Join(outputDir, entry.Name())
		mediaType := r.detectMediaType(entry.Name())

		mediaFiles = append(mediaFiles, entity.MediaFile{
			FilePath:  filePath,
			FileName:  entry.Name(),
			FileSize:  info.Size(),
			MediaType: mediaType,
		})
	}

	if len(mediaFiles) == 0 {
		return &entity.MediaProcessResult{
			Success: false,
			Error:   fmt.Errorf("failed to download media"),
		}, fmt.Errorf("no media downloaded")
	}

	return &entity.MediaProcessResult{
		Success: true,
		Files:   mediaFiles,
	}, nil
}

func (r *DownloadRepository) detectMediaType(filename string) entity.MediaType {
	ext := strings.ToLower(filepath.Ext(filename))

	// Check video extensions from config
	for _, ve := range r.environment.VideoDownloaderConfiguration.SupportedExtensions {
		if ext == strings.ToLower(ve) {
			return entity.MediaTypeVideo
		}
	}

	// Check image extensions from config
	for _, ie := range r.environment.PhotoDownloaderConfiguration.SupportedExtensions {
		if ext == strings.ToLower(ie) {
			return entity.MediaTypeImage
		}
	}

	// Check audio extensions from config
	for _, ae := range r.environment.AudioDownloaderConfiguration.SupportedExtensions {
		if ext == strings.ToLower(ae) {
			return entity.MediaTypeAudio
		}
	}

	// Default to video if unknown
	return entity.MediaTypeVideo
}

// applyYtdlpOptions applies yt-dlp configuration options from the environment
func (r *DownloadRepository) applyYtdlpOptions(dl *ytdlp.Command) *ytdlp.Command {
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

	// Custom HTTP headers (including User-Agent)
	for _, header := range config.CustomHeaders {
		if header != "" {
			dl = dl.AddHeaders(header)
		}
	}

	return dl
}
