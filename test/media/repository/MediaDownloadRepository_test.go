package repository_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/lrstanley/go-ytdlp"
	"tg-downloader/env"
	"tg-downloader/src/core/logger"
	"tg-downloader/src/features/media/data/repository"
	"tg-downloader/src/features/media/domain/entity"
)

// newTestLogger returns a no-op logger (debug disabled) suitable for tests.
func newTestLogger() *logger.Logger {
	return logger.NewLogger([]logger.ILoggerStrategy{logger.NewDebugLoggerStrategy(false)})
}

// newTestEnv builds a minimal env.TGDownloader for testing with the given maxFileSizeMB.
func newTestEnv(maxFileSizeMB int) env.TGDownloader {
	return env.TGDownloader{
		VideoDownloaderConfiguration: env.VideoDownloaderConfiguration{
			MaxFileSizeMB: maxFileSizeMB,
		},
	}
}

// createFileOfSize writes a file of exactly sizeBytes into outputDir.
func createFileOfSize(t *testing.T, outputDir, name string, sizeBytes int64) string {
	t.Helper()
	path := filepath.Join(outputDir, name)
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("createFileOfSize: %v", err)
	}
	defer f.Close()
	if sizeBytes > 0 {
		if err := f.Truncate(sizeBytes); err != nil {
			t.Fatalf("createFileOfSize truncate: %v", err)
		}
	}
	return path
}

// mockRunner tracks calls and executes per-call behavior functions.
type mockRunner struct {
	calls     int
	behaviors []func(outputDir string) error
	outputDir string // set before use
}

func (m *mockRunner) run(cmd *ytdlp.Command, url string) error {
	idx := m.calls
	m.calls++
	if idx < len(m.behaviors) {
		return m.behaviors[idx](m.outputDir)
	}
	return fmt.Errorf("unexpected runner call %d", idx)
}

// genericPlatform returns a PlatformInfo with the given strategies and no extractor args.
func genericPlatform(strategies []env.FormatStrategy) entity.PlatformInfo {
	return entity.PlatformInfo{
		Name:       "Test",
		Strategies: strategies,
	}
}

// --- TestFallbackOnOversize ---
func TestFallbackOnOversize(t *testing.T) {
	outputDir := t.TempDir()
	const mb = int64(1024 * 1024)

	strats := []env.FormatStrategy{
		{Name: "strategy-1", Format: "best"},
		{Name: "strategy-2", Format: "worst"},
	}
	cfg := newTestEnv(10)

	mock := &mockRunner{outputDir: outputDir}
	mock.behaviors = []func(string) error{
		func(dir string) error {
			createFileOfSize(t, dir, "video1.mp4", 20*mb)
			return nil
		},
		func(dir string) error {
			createFileOfSize(t, dir, "video2.mp4", 5*mb)
			return nil
		},
	}

	repo := repository.NewMediaDownloadRepositoryWithRunner(cfg, newTestLogger(), mock.run)
	result, err := repo.DownloadMedia("https://example.com/video", outputDir, genericPlatform(strats))

	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}
	if !result.Success {
		t.Fatalf("expected result.Success=true, got false (error: %v)", result.Error)
	}
	if result.FileSize > 10*mb {
		t.Errorf("result.FileSize=%d > 10MB limit", result.FileSize)
	}
	if mock.calls != 2 {
		t.Errorf("expected 2 runner calls, got %d", mock.calls)
	}
}

// --- TestFallbackOnYtdlpError ---
func TestFallbackOnYtdlpError(t *testing.T) {
	outputDir := t.TempDir()
	const mb = int64(1024 * 1024)

	strats := []env.FormatStrategy{
		{Name: "strategy-1", Format: "best"},
		{Name: "strategy-2", Format: "worst"},
	}
	cfg := newTestEnv(10)

	mock := &mockRunner{outputDir: outputDir}
	mock.behaviors = []func(string) error{
		func(dir string) error {
			return fmt.Errorf("yt-dlp ERROR: some download error")
		},
		func(dir string) error {
			createFileOfSize(t, dir, "video.mp4", 5*mb)
			return nil
		},
	}

	repo := repository.NewMediaDownloadRepositoryWithRunner(cfg, newTestLogger(), mock.run)
	result, err := repo.DownloadMedia("https://example.com/video", outputDir, genericPlatform(strats))

	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}
	if !result.Success {
		t.Fatalf("expected result.Success=true, got false (error: %v)", result.Error)
	}
	if mock.calls != 2 {
		t.Errorf("expected 2 runner calls, got %d", mock.calls)
	}
}

// --- TestCleanupBeforeRetry ---
func TestCleanupBeforeRetry(t *testing.T) {
	outputDir := t.TempDir()
	const mb = int64(1024 * 1024)

	strats := []env.FormatStrategy{
		{Name: "strategy-1", Format: "best"},
		{Name: "strategy-2", Format: "worst"},
	}
	cfg := newTestEnv(10)

	var oversizedPath string

	mock := &mockRunner{outputDir: outputDir}
	mock.behaviors = []func(string) error{
		func(dir string) error {
			oversizedPath = createFileOfSize(t, dir, "oversized.mp4", 20*mb)
			return nil
		},
		func(dir string) error {
			// At this point the oversized file must have been cleaned up
			if _, statErr := os.Stat(oversizedPath); statErr == nil {
				return fmt.Errorf("oversized file still exists before strategy-2 ran: %s", oversizedPath)
			}
			createFileOfSize(t, dir, "small.mp4", 5*mb)
			return nil
		},
	}

	repo := repository.NewMediaDownloadRepositoryWithRunner(cfg, newTestLogger(), mock.run)
	result, err := repo.DownloadMedia("https://example.com/video", outputDir, genericPlatform(strats))

	if err != nil {
		t.Fatalf("TestCleanupBeforeRetry: expected success, got error: %v", err)
	}
	if !result.Success {
		t.Fatalf("TestCleanupBeforeRetry: expected result.Success=true, got false: %v", result.Error)
	}
}

// --- TestBuildStrategiesWithExtractorArgs ---
func TestBuildStrategiesWithExtractorArgs(t *testing.T) {
	outputDir := t.TempDir()

	platform := entity.PlatformInfo{
		Name: "YouTube Shorts",
		Strategies: []env.FormatStrategy{
			{Name: "yt-strategy-1", Format: "best"},
			{Name: "yt-strategy-2", Format: "worst"},
		},
		ExtractorArgs: []string{"youtube:player_client=android_sdkless"},
	}
	cfg := newTestEnv(10)

	calls := 0
	runner := func(cmd *ytdlp.Command, url string) error {
		calls++
		return fmt.Errorf("simulated failure")
	}

	repo := repository.NewMediaDownloadRepositoryWithRunner(cfg, newTestLogger(), runner)
	result, _ := repo.DownloadMedia("https://youtube.com/watch?v=test", outputDir, platform)

	if result.Success {
		t.Fatal("expected all strategies to fail")
	}
	if calls != len(platform.Strategies) {
		t.Errorf("expected %d runner calls, got %d", len(platform.Strategies), calls)
	}
}

// --- TestBuildStrategiesWithoutExtractorArgs ---
func TestBuildStrategiesWithoutExtractorArgs(t *testing.T) {
	outputDir := t.TempDir()

	platform := entity.PlatformInfo{
		Name: "TikTok",
		Strategies: []env.FormatStrategy{
			{Name: "generic-1", Format: "best"},
			{Name: "generic-2", Format: "worst"},
			{Name: "generic-3", Format: "worst"},
		},
	}
	cfg := newTestEnv(10)

	calls := 0
	runner := func(cmd *ytdlp.Command, url string) error {
		calls++
		return fmt.Errorf("simulated failure")
	}

	repo := repository.NewMediaDownloadRepositoryWithRunner(cfg, newTestLogger(), runner)
	result, _ := repo.DownloadMedia("https://tiktok.com/video", outputDir, platform)

	if result.Success {
		t.Fatal("expected all strategies to fail")
	}
	if calls != len(platform.Strategies) {
		t.Errorf("expected %d runner calls, got %d", len(platform.Strategies), calls)
	}
}

// --- TestClassifyOutputDirImagesOnly ---
func TestClassifyOutputDirImagesOnly(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"slide1.jpg", "slide2.png", "slide3.webp"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("test"), 0644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
	}

	items, err := repository.ClassifyOutputDir(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(items))
	}
	for _, item := range items {
		if item.Type != entity.MediaTypePhoto {
			t.Errorf("expected MediaTypePhoto, got %v for %s", item.Type, item.FilePath)
		}
	}
}

// --- TestClassifyOutputDirVideosOnly ---
func TestClassifyOutputDirVideosOnly(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"clip.mp4", "clip2.mkv"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("test"), 0644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
	}

	items, err := repository.ClassifyOutputDir(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	for _, item := range items {
		if item.Type != entity.MediaTypeVideo {
			t.Errorf("expected MediaTypeVideo, got %v for %s", item.Type, item.FilePath)
		}
	}
}

// --- TestClassifyOutputDirMixed ---
func TestClassifyOutputDirMixed(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"photo.jpg", "clip.mp4", "photo2.webp"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("test"), 0644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
	}

	items, err := repository.ClassifyOutputDir(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(items))
	}

	var photos, videos int
	for _, item := range items {
		if item.Type == entity.MediaTypePhoto {
			photos++
		} else if item.Type == entity.MediaTypeVideo {
			videos++
		}
	}
	if photos != 2 {
		t.Errorf("expected 2 photos, got %d", photos)
	}
	if videos != 1 {
		t.Errorf("expected 1 video, got %d", videos)
	}
}

// --- TestClassifyOutputDirEmpty ---
func TestClassifyOutputDirEmpty(t *testing.T) {
	dir := t.TempDir()

	items, err := repository.ClassifyOutputDir(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("expected 0 items, got %d", len(items))
	}
}

// --- TestClassifyOutputDirIgnoresUnknown ---
func TestClassifyOutputDirIgnoresUnknown(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"readme.txt", "data.json", "image.gif"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("test"), 0644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
	}

	items, err := repository.ClassifyOutputDir(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("expected 0 items for unknown extensions, got %d", len(items))
	}
}

// --- TestClassifyOutputDirIgnoresSubdirs ---
func TestClassifyOutputDirIgnoresSubdirs(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "photos.jpg"), 0755); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}

	items, err := repository.ClassifyOutputDir(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("expected 0 items (subdirs skipped), got %d", len(items))
	}
}

// --- TestClassifyOutputDirCaseInsensitive ---
func TestClassifyOutputDirCaseInsensitive(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"IMAGE.JPG", "Photo.PNG"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("test"), 0644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
	}

	items, err := repository.ClassifyOutputDir(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items (case-insensitive), got %d", len(items))
	}
	for _, item := range items {
		if item.Type != entity.MediaTypePhoto {
			t.Errorf("expected MediaTypePhoto, got %v", item.Type)
		}
	}
}

// --- TestClassifyOutputDirModtimeOrder ---
func TestClassifyOutputDirModtimeOrder(t *testing.T) {
	dir := t.TempDir()

	// Create files
	path1 := filepath.Join(dir, "slide1.jpg")
	path2 := filepath.Join(dir, "slide2.jpg")
	path3 := filepath.Join(dir, "slide3.jpg")
	for _, p := range []string{path1, path2, path3} {
		if err := os.WriteFile(p, []byte("test"), 0644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
	}

	// Set modtimes: oldest first
	now := time.Now()
	os.Chtimes(path1, now.Add(-3*time.Second), now.Add(-3*time.Second))
	os.Chtimes(path2, now.Add(-2*time.Second), now.Add(-2*time.Second))
	os.Chtimes(path3, now.Add(-1*time.Second), now.Add(-1*time.Second))

	items, err := repository.ClassifyOutputDir(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(items))
	}

	// Verify ordering: items should be sorted by modtime ascending
	if items[0].Order != 0 || items[1].Order != 1 || items[2].Order != 2 {
		t.Errorf("expected Order fields 0,1,2 got %d,%d,%d", items[0].Order, items[1].Order, items[2].Order)
	}
	// Verify slide1.jpg is first (oldest modtime)
	if filepath.Base(items[0].FilePath) != "slide1.jpg" {
		t.Errorf("expected slide1.jpg first (oldest), got %s", filepath.Base(items[0].FilePath))
	}
	if filepath.Base(items[2].FilePath) != "slide3.jpg" {
		t.Errorf("expected slide3.jpg last (newest), got %s", filepath.Base(items[2].FilePath))
	}
}
