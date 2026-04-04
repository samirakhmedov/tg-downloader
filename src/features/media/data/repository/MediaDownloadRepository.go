package repository

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"tg-downloader/env"
	"tg-downloader/src/core"
	"tg-downloader/src/core/logger"
	"tg-downloader/src/features/media/data/repository/strategy"
	"tg-downloader/src/features/media/domain/entity"
	iRepo "tg-downloader/src/features/media/domain/repository"
	"time"

	"github.com/lrstanley/go-ytdlp"
)

// YtdlpRunnerFunc is the signature for running a yt-dlp command.
// Production code uses makeYtdlpRunner; tests inject a mock.
type YtdlpRunnerFunc func(cmd *ytdlp.Command, url string) error

// makeYtdlpRunner creates a YtdlpRunnerFunc with the given timeout.
func makeYtdlpRunner(timeout time.Duration) YtdlpRunnerFunc {
	return func(cmd *ytdlp.Command, url string) error {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		_, err := cmd.Run(ctx, url)
		return err
	}
}

type MediaDownloadRepository struct {
	environment env.TGDownloader
	logger      *logger.Logger
	runYtdlp    YtdlpRunnerFunc
}

func NewMediaDownloadRepository(environment env.TGDownloader, log *logger.Logger) iRepo.IMediaDownloadRepository {
	timeout := time.Duration(environment.VideoDownloaderConfiguration.DownloadTimeoutSeconds) * time.Second
	return &MediaDownloadRepository{
		environment: environment,
		logger:      log,
		runYtdlp:    makeYtdlpRunner(timeout),
	}
}

// NewMediaDownloadRepositoryWithRunner creates a repository with a custom yt-dlp runner (for testing).
func NewMediaDownloadRepositoryWithRunner(environment env.TGDownloader, log *logger.Logger, runner YtdlpRunnerFunc) *MediaDownloadRepository {
	return &MediaDownloadRepository{
		environment: environment,
		logger:      log,
		runYtdlp:    runner,
	}
}

func (r *MediaDownloadRepository) ValidateURL(url string) (bool, entity.PlatformInfo, error) {
	// First check if it's a valid URL format
	urlRegex := regexp.MustCompile(core.URLRegexPattern)
	if !urlRegex.MatchString(url) {
		return false, entity.PlatformInfo{}, fmt.Errorf("invalid URL format")
	}

	// Check against supported patterns
	for _, linkPattern := range r.environment.CommandConfiguration.SupportedLinks {
		matched, err := regexp.MatchString(linkPattern.Pattern, url)
		if err != nil {
			continue
		}
		if matched {
			return true, entity.PlatformInfo{
				Name:          linkPattern.Name,
				Strategies:    linkPattern.Strategies,
				ExtractorArgs: linkPattern.ExtractorArgs,
				Username:      linkPattern.Username,
				Password:      linkPattern.Password,
			}, nil
		}
	}

	// Build error message with supported formats
	var supportedFormats string
	for i, linkPattern := range r.environment.CommandConfiguration.SupportedLinks {
		if i > 0 {
			supportedFormats += "\n"
		}
		supportedFormats += fmt.Sprintf("* %s: %s", linkPattern.Name, linkPattern.Example)
	}

	return false, entity.PlatformInfo{}, fmt.Errorf("unsupported video format. Supported formats:\n%s", supportedFormats)
}

// buildStrategies constructs the ordered strategy slice from the matched platform's config.
func (r *MediaDownloadRepository) buildStrategies(platform entity.PlatformInfo) []strategy.IDownloadStrategy {
	strategies := make([]strategy.IDownloadStrategy, 0, len(platform.Strategies))
	for _, s := range platform.Strategies {
		strategies = append(strategies, strategy.NewDownloadStrategy(s.Name, s.Format, platform.ExtractorArgs))
	}
	return strategies
}

// findDownloadedFile scans outputDir for the most recently modified file and returns its path, size, and existence.
func (r *MediaDownloadRepository) findDownloadedFile(outputDir string) (string, int64, bool) {
	entries, err := os.ReadDir(outputDir)
	if err != nil {
		return "", 0, false
	}

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

	if latestPath == "" {
		return "", 0, false
	}
	return latestPath, latestFile.Size(), true
}

func (r *MediaDownloadRepository) DownloadMedia(url string, outputDir string, platform entity.PlatformInfo) (*entity.DownloadResult, error) {
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return &entity.DownloadResult{
			Success: false,
			Error:   fmt.Errorf("failed to create output directory: %w", err),
		}, err
	}

	strategies := r.buildStrategies(platform)
	maxBytes := int64(r.environment.VideoDownloaderConfiguration.MaxFileSizeMB) * 1024 * 1024

	var lastErr error
	var prevFile string

	for _, strat := range strategies {
		// Clean up previous oversized file before starting next attempt
		if prevFile != "" {
			os.Remove(prevFile)
			prevFile = ""
		}

		r.logger.Info(fmt.Sprintf("Attempting strategy: %s", strat.Name()))

		cmd := ytdlp.New().
			SetExecutable(r.environment.CommonDownloaderConfiguration.YtdlpExecutablePath).
			Output(filepath.Join(outputDir, "%(title)s.%(ext)s")).
			NoCheckCertificates()

		// Apply strategy-specific format + extractor args first
		cmd = strat.Apply(cmd)
		// Apply global options (IPv4, sleep, headers, generic extractorArgs) second
		cmd = r.applyYtdlpOptions(cmd)
		// Apply per-platform authentication credentials
		if platform.Username != nil && *platform.Username != "" {
			cmd = cmd.Username(*platform.Username)
		}
		if platform.Password != nil && *platform.Password != "" {
			cmd = cmd.Password(*platform.Password)
		}

		if err := r.runYtdlp(cmd, url); err != nil {
			lastErr = err
			r.logger.Warn(fmt.Sprintf("Strategy %s failed: %v", strat.Name(), err))
			continue
		}

		path, size, found := r.findDownloadedFile(outputDir)
		if !found {
			lastErr = fmt.Errorf("no file produced by strategy %s", strat.Name())
			r.logger.Warn(fmt.Sprintf("No file produced by strategy %s", strat.Name()))
			continue
		}

		if size <= maxBytes {
			r.logger.Info(fmt.Sprintf("Strategy %s succeeded, file size %dMB", strat.Name(), size/1024/1024))
			return &entity.DownloadResult{
				Success:  true,
				FilePath: path,
				FileName: filepath.Base(path),
				FileSize: size,
			}, nil
		}

		prevFile = path
		lastErr = fmt.Errorf("file %dMB exceeds %dMB limit at strategy %s",
			size/1024/1024, maxBytes/1024/1024, strat.Name())
		r.logger.Info(fmt.Sprintf("File too large (%dMB), trying lower quality", size/1024/1024))
	}

	// Clean up the last oversized file if any
	if prevFile != "" {
		os.Remove(prevFile)
	}
	return &entity.DownloadResult{Success: false, Error: lastErr}, lastErr
}

// ClassifyOutputDir scans a directory for downloaded media files, classifies them
// by extension (images vs videos), and returns them sorted by modification time ascending.
func ClassifyOutputDir(dir string) ([]entity.MediaItem, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	imageExts := map[string]bool{".jpg": true, ".jpeg": true, ".png": true, ".webp": true}
	videoExts := map[string]bool{".mp4": true, ".mkv": true, ".webm": true}

	type fileWithTime struct {
		path    string
		modTime time.Time
		mType   entity.MediaType
	}
	var files []fileWithTime

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		var mType entity.MediaType
		if imageExts[ext] {
			mType = entity.MediaTypePhoto
		} else if videoExts[ext] {
			mType = entity.MediaTypeVideo
		} else {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		files = append(files, fileWithTime{
			path:    filepath.Join(dir, entry.Name()),
			modTime: info.ModTime(),
			mType:   mType,
		})
	}

	// Sort by modification time ascending (preserves yt-dlp download order)
	sort.Slice(files, func(i, j int) bool {
		return files[i].modTime.Before(files[j].modTime)
	})

	items := make([]entity.MediaItem, len(files))
	for i, f := range files {
		items[i] = entity.MediaItem{
			FilePath: f.path,
			Type:     f.mType,
			Order:    i,
		}
	}
	return items, nil
}

// applyYtdlpOptions applies yt-dlp configuration options from the environment
func (r *MediaDownloadRepository) applyYtdlpOptions(dl *ytdlp.Command) *ytdlp.Command {
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
