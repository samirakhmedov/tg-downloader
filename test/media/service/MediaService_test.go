package service_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"tg-downloader/env"
	"tg-downloader/src/core/logger"
	botEntity "tg-downloader/src/features/bot/domain/entity"
	botRepo "tg-downloader/src/features/bot/domain/repository"
	"tg-downloader/src/features/media/data/service"
	"tg-downloader/src/features/media/domain/entity"
	iMediaRepo "tg-downloader/src/features/media/domain/repository"
)

// --- Logger helper ---

func newTestLogger() *logger.Logger {
	return logger.NewLogger([]logger.ILoggerStrategy{logger.NewDebugLoggerStrategy(false)})
}

// --- Mock implementations ---

// mockTaskRepository is a minimal ITaskRepository that returns tasks in order.
type mockTaskRepository struct {
	tasks         []*botEntity.Task
	nextIndex     int
	markedIDs     []int
	markShouldErr bool
}

func (m *mockTaskRepository) CreateTask(link string, groupID int64, messageID int) (*botEntity.Task, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockTaskRepository) GetNextTask() (*botEntity.Task, error) {
	if m.nextIndex >= len(m.tasks) {
		return nil, fmt.Errorf("no more tasks")
	}
	task := m.tasks[m.nextIndex]
	m.nextIndex++
	return task, nil
}

func (m *mockTaskRepository) MarkTaskInProgress(id int) error {
	if m.markShouldErr {
		return fmt.Errorf("mark error")
	}
	m.markedIDs = append(m.markedIDs, id)
	return nil
}

func (m *mockTaskRepository) DeleteTask(id int) error {
	return nil
}

func (m *mockTaskRepository) FindTaskByLink(link string) (*botEntity.Task, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockTaskRepository) AddGroupToTask(taskID int, groupID int64, messageID int) error {
	return fmt.Errorf("not implemented")
}

// Ensure mockTaskRepository implements ITaskRepository.
var _ botRepo.ITaskRepository = (*mockTaskRepository)(nil)

// mockDownloadRepository is a configurable IMediaDownloadRepository stub.
type mockDownloadRepository struct {
	validateResult        bool
	validatePlatform      entity.PlatformInfo
	validateErr           error
	downloadResult        *entity.DownloadResult
	downloadErr           error
	createFileInOutputDir bool
	filesToCreate         []string // list of filenames to create (e.g., "image.jpg", "clip.mp4")
}

func (m *mockDownloadRepository) ValidateURL(url string) (bool, entity.PlatformInfo, error) {
	return m.validateResult, m.validatePlatform, m.validateErr
}

func (m *mockDownloadRepository) DownloadMedia(url string, outputDir string, platform entity.PlatformInfo) (*entity.DownloadResult, error) {
	if m.downloadErr != nil {
		return m.downloadResult, m.downloadErr
	}
	if len(m.filesToCreate) > 0 {
		for i, name := range m.filesToCreate {
			filePath := filepath.Join(outputDir, name)
			os.WriteFile(filePath, []byte("fake content"), 0644)
			// Stagger modification times so ClassifyOutputDir ordering is deterministic
			modTime := time.Now().Add(time.Duration(i) * time.Second)
			os.Chtimes(filePath, modTime, modTime)
		}
		result := *m.downloadResult
		result.FilePath = filepath.Join(outputDir, m.filesToCreate[0])
		return &result, nil
	}
	if m.createFileInOutputDir {
		filePath := filepath.Join(outputDir, "test-video.mp4")
		os.WriteFile(filePath, []byte("fake video content"), 0644)
		result := *m.downloadResult
		result.FilePath = filePath
		return &result, nil
	}
	return m.downloadResult, nil
}

// Ensure mockDownloadRepository implements IMediaDownloadRepository.
var _ iMediaRepo.IMediaDownloadRepository = (*mockDownloadRepository)(nil)

// mockMediaUploadRepository is a configurable IMediaUploadRepository stub.
type mockMediaUploadRepository struct {
	uploadVideoCalls int
	uploadPhotoCalls int
	uploadAlbumCalls int
	lastAlbumItems   []entity.MediaItem
	videoErr         error
	photoErr         error
	albumErr         error
}

func (m *mockMediaUploadRepository) UploadVideo(ctx context.Context, filePath string, groupID int64) error {
	m.uploadVideoCalls++
	return m.videoErr
}

func (m *mockMediaUploadRepository) UploadPhoto(ctx context.Context, filePath string, groupID int64) error {
	m.uploadPhotoCalls++
	return m.photoErr
}

func (m *mockMediaUploadRepository) UploadAlbum(ctx context.Context, items []entity.MediaItem, groupID int64) error {
	m.uploadAlbumCalls++
	m.lastAlbumItems = items
	return m.albumErr
}

// Ensure mockMediaUploadRepository implements IMediaUploadRepository.
var _ iMediaRepo.IMediaUploadRepository = (*mockMediaUploadRepository)(nil)

// newTestEnvWithWorkerCount builds a minimal env.TGDownloader with the given worker count.
func newTestEnvWithWorkerCount(workerCount int) env.TGDownloader {
	return env.TGDownloader{
		WorkerConfiguration: env.WorkerConfiguration{
			WorkerCount:         workerCount,
			TaskPollingInterval: 60, // long interval so ticker never fires during test
		},
		VideoDownloaderConfiguration: env.VideoDownloaderConfiguration{
			DownloadTimeoutSeconds: 300,
			UploadTimeoutSeconds:   300,
		},
	}
}

// --- ClassifyDownloadError tests ---

func TestClassifyDownloadErrorNil(t *testing.T) {
	got := service.ClassifyDownloadError(nil)
	want := "Download failed. Please try again later."
	if got != want {
		t.Errorf("ClassifyDownloadError(nil) = %q, want %q", got, want)
	}
}

func TestClassifyDownloadErrorPrivateVideo(t *testing.T) {
	err := errors.New("ERROR: Private video. Sign in if you have access to it.")
	got := service.ClassifyDownloadError(err)
	want := "This video is private and cannot be downloaded."
	if got != want {
		t.Errorf("ClassifyDownloadError(%q) = %q, want %q", err, got, want)
	}
}

func TestClassifyDownloadErrorUnsupportedURL(t *testing.T) {
	err := errors.New("ERROR: Unsupported URL: https://example.com/video")
	got := service.ClassifyDownloadError(err)
	want := "This URL is not supported."
	if got != want {
		t.Errorf("ClassifyDownloadError(%q) = %q, want %q", err, got, want)
	}
}

func TestClassifyDownloadErrorNetworkError(t *testing.T) {
	err := errors.New("socket.gaierror: [Errno -2] Temporary failure in name resolution")
	got := service.ClassifyDownloadError(err)
	want := "Download failed due to a network error. Please try again."
	if got != want {
		t.Errorf("ClassifyDownloadError(%q) = %q, want %q", err, got, want)
	}
}

func TestClassifyDownloadErrorGeneric(t *testing.T) {
	err := errors.New("some unknown unexpected error from yt-dlp")
	got := service.ClassifyDownloadError(err)
	want := "Download failed. Please try again later."
	if got != want {
		t.Errorf("ClassifyDownloadError(%q) = %q, want %q", err, got, want)
	}
}

func TestClassifyDownloadErrorTimeout(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{"string match", errors.New("download failed: context deadline exceeded")},
		{"wrapped context error", fmt.Errorf("wrapped: %w", context.DeadlineExceeded)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := service.ClassifyDownloadError(tt.err)
			want := "Download timed out. Please try again later."
			if got != want {
				t.Errorf("ClassifyDownloadError(%q) = %q, want %q", tt.err, got, want)
			}
		})
	}
}

// --- TestScheduleStopsWhenQueueFull ---
func TestScheduleStopsWhenQueueFull(t *testing.T) {
	tasks := []*botEntity.Task{
		{ID: 1, Link: "https://example.com/1", GroupIDs: []int64{100}, StatusMessageIDs: map[int64]int{100: 1}},
		{ID: 2, Link: "https://example.com/2", GroupIDs: []int64{100}, StatusMessageIDs: map[int64]int{100: 2}},
		{ID: 3, Link: "https://example.com/3", GroupIDs: []int64{100}, StatusMessageIDs: map[int64]int{100: 3}},
	}

	taskRepo := &mockTaskRepository{tasks: tasks}

	// WorkerCount=1 means taskQueue capacity=1
	cfg := newTestEnvWithWorkerCount(1)

	svc := service.NewMediaService(
		cfg,
		taskRepo,
		&mockDownloadRepository{},
		&mockMediaUploadRepository{},
		newTestLogger(),
	)

	svc.ScheduleAvailableTasks()

	// Tasks 1 and 2 should be marked in_progress; task 3 must NOT be marked.
	if len(taskRepo.markedIDs) != 2 {
		t.Errorf("expected 2 tasks marked in_progress (tasks 1+2), got %d (marked IDs: %v)", len(taskRepo.markedIDs), taskRepo.markedIDs)
	}
	for _, id := range taskRepo.markedIDs {
		if id == 3 {
			t.Errorf("task 3 must NOT be marked in_progress — loop should have stopped before reaching it")
		}
	}
}

// --- Temp dir cleanup tests ---

func TestProcessTaskTempDirCleanup(t *testing.T) {
	downloadMock := &mockDownloadRepository{
		validateResult:        true,
		validatePlatform:      entity.PlatformInfo{Name: "Test"},
		downloadResult:        &entity.DownloadResult{Success: true, FileName: "test.mp4", FileSize: 1024},
		createFileInOutputDir: true,
	}
	mediaUploadMock := &mockMediaUploadRepository{}

	tasks := []*botEntity.Task{
		{ID: 1, Link: "https://example.com/video", GroupIDs: []int64{100}, StatusMessageIDs: map[int64]int{100: 1}},
	}
	taskRepo := &mockTaskRepository{tasks: tasks}
	cfg := newTestEnvWithWorkerCount(1)

	svc := service.NewMediaService(cfg, taskRepo, downloadMock, mediaUploadMock, newTestLogger())
	svc.StartWorkers()
	svc.ScheduleAvailableTasks()

	// Give worker time to process
	time.Sleep(500 * time.Millisecond)
	svc.StopWorkers()

	// Verify: no temp directories matching tg-dl-* pattern remain in OS temp dir
	matches, _ := filepath.Glob(filepath.Join(os.TempDir(), "tg-dl-*"))
	if len(matches) > 0 {
		t.Errorf("expected no tg-dl-* temp directories after successful processTask, found: %v", matches)
	}
}

func TestProcessTaskTempDirCleanedOnDownloadFailure(t *testing.T) {
	downloadMock := &mockDownloadRepository{
		validateResult:   true,
		validatePlatform: entity.PlatformInfo{Name: "Test"},
		downloadResult:   &entity.DownloadResult{Success: false, Error: errors.New("download failed")},
		downloadErr:      errors.New("download failed"),
	}
	mediaUploadMock := &mockMediaUploadRepository{}

	tasks := []*botEntity.Task{
		{ID: 1, Link: "https://example.com/video", GroupIDs: []int64{100}, StatusMessageIDs: map[int64]int{100: 1}},
	}
	taskRepo := &mockTaskRepository{tasks: tasks}
	cfg := newTestEnvWithWorkerCount(1)

	svc := service.NewMediaService(cfg, taskRepo, downloadMock, mediaUploadMock, newTestLogger())
	svc.StartWorkers()
	svc.ScheduleAvailableTasks()

	time.Sleep(500 * time.Millisecond)
	svc.StopWorkers()

	matches, _ := filepath.Glob(filepath.Join(os.TempDir(), "tg-dl-*"))
	if len(matches) > 0 {
		t.Errorf("expected no tg-dl-* temp directories after download failure, found: %v", matches)
	}
}

func TestUploadFailureLeavesNoTempFiles(t *testing.T) {
	downloadMock := &mockDownloadRepository{
		validateResult:        true,
		validatePlatform:      entity.PlatformInfo{Name: "Test"},
		downloadResult:        &entity.DownloadResult{Success: true, FileName: "test.mp4", FileSize: 1024},
		createFileInOutputDir: true,
	}
	mediaUploadMock := &mockMediaUploadRepository{videoErr: errors.New("upload failed: connection timeout")}

	tasks := []*botEntity.Task{
		{ID: 1, Link: "https://example.com/video", GroupIDs: []int64{100}, StatusMessageIDs: map[int64]int{100: 1}},
	}
	taskRepo := &mockTaskRepository{tasks: tasks}
	cfg := newTestEnvWithWorkerCount(1)

	svc := service.NewMediaService(cfg, taskRepo, downloadMock, mediaUploadMock, newTestLogger())
	svc.StartWorkers()
	svc.ScheduleAvailableTasks()

	time.Sleep(500 * time.Millisecond)
	svc.StopWorkers()

	matches, _ := filepath.Glob(filepath.Join(os.TempDir(), "tg-dl-*"))
	if len(matches) > 0 {
		t.Errorf("expected no tg-dl-* temp directories after upload failure, found: %v", matches)
	}
}

// --- Routing tests ---

func TestProcessTaskSingleImageRoute(t *testing.T) {
	downloadMock := &mockDownloadRepository{
		validateResult:   true,
		validatePlatform: entity.PlatformInfo{Name: "Instagram Posts"},
		downloadResult:   &entity.DownloadResult{Success: true, FileName: "photo.jpg", FileSize: 1024},
		filesToCreate:    []string{"photo.jpg"},
	}
	mediaUploadMock := &mockMediaUploadRepository{}

	tasks := []*botEntity.Task{
		{ID: 1, Link: "https://instagram.com/p/abc123", GroupIDs: []int64{100}, StatusMessageIDs: map[int64]int{100: 1}},
	}
	taskRepo := &mockTaskRepository{tasks: tasks}
	cfg := newTestEnvWithWorkerCount(1)

	svc := service.NewMediaService(cfg, taskRepo, downloadMock, mediaUploadMock, newTestLogger())
	svc.StartWorkers()
	svc.ScheduleAvailableTasks()

	time.Sleep(500 * time.Millisecond)
	svc.StopWorkers()

	if mediaUploadMock.uploadPhotoCalls != 1 {
		t.Errorf("expected 1 UploadPhoto call, got %d", mediaUploadMock.uploadPhotoCalls)
	}
	if mediaUploadMock.uploadAlbumCalls != 0 {
		t.Errorf("expected 0 UploadAlbum calls, got %d", mediaUploadMock.uploadAlbumCalls)
	}
	if mediaUploadMock.uploadVideoCalls != 0 {
		t.Errorf("expected 0 UploadVideo calls, got %d", mediaUploadMock.uploadVideoCalls)
	}
}

func TestProcessTaskMultiImageRoute(t *testing.T) {
	downloadMock := &mockDownloadRepository{
		validateResult:   true,
		validatePlatform: entity.PlatformInfo{Name: "Instagram Posts"},
		downloadResult:   &entity.DownloadResult{Success: true, FileName: "slide1.jpg", FileSize: 1024},
		filesToCreate:    []string{"slide1.jpg", "slide2.png", "slide3.webp"},
	}
	mediaUploadMock := &mockMediaUploadRepository{}

	tasks := []*botEntity.Task{
		{ID: 1, Link: "https://instagram.com/p/abc123", GroupIDs: []int64{100}, StatusMessageIDs: map[int64]int{100: 1}},
	}
	taskRepo := &mockTaskRepository{tasks: tasks}
	cfg := newTestEnvWithWorkerCount(1)

	svc := service.NewMediaService(cfg, taskRepo, downloadMock, mediaUploadMock, newTestLogger())
	svc.StartWorkers()
	svc.ScheduleAvailableTasks()

	time.Sleep(500 * time.Millisecond)
	svc.StopWorkers()

	if mediaUploadMock.uploadAlbumCalls != 1 {
		t.Errorf("expected 1 UploadAlbum call, got %d", mediaUploadMock.uploadAlbumCalls)
	}
	if len(mediaUploadMock.lastAlbumItems) != 3 {
		t.Errorf("expected 3 album items, got %d", len(mediaUploadMock.lastAlbumItems))
	}
	if mediaUploadMock.uploadPhotoCalls != 0 {
		t.Errorf("expected 0 UploadPhoto calls, got %d", mediaUploadMock.uploadPhotoCalls)
	}
	if mediaUploadMock.uploadVideoCalls != 0 {
		t.Errorf("expected 0 UploadVideo calls, got %d", mediaUploadMock.uploadVideoCalls)
	}
}

func TestProcessTaskMixedMediaRoute(t *testing.T) {
	downloadMock := &mockDownloadRepository{
		validateResult:   true,
		validatePlatform: entity.PlatformInfo{Name: "Instagram Posts"},
		downloadResult:   &entity.DownloadResult{Success: true, FileName: "photo1.jpg", FileSize: 1024},
		filesToCreate:    []string{"photo1.jpg", "clip.mp4", "photo2.webp"},
	}
	mediaUploadMock := &mockMediaUploadRepository{}

	tasks := []*botEntity.Task{
		{ID: 1, Link: "https://instagram.com/p/abc123", GroupIDs: []int64{100}, StatusMessageIDs: map[int64]int{100: 1}},
	}
	taskRepo := &mockTaskRepository{tasks: tasks}
	cfg := newTestEnvWithWorkerCount(1)

	svc := service.NewMediaService(cfg, taskRepo, downloadMock, mediaUploadMock, newTestLogger())
	svc.StartWorkers()
	svc.ScheduleAvailableTasks()

	time.Sleep(500 * time.Millisecond)
	svc.StopWorkers()

	if mediaUploadMock.uploadAlbumCalls != 1 {
		t.Errorf("expected 1 UploadAlbum call, got %d", mediaUploadMock.uploadAlbumCalls)
	}
	if len(mediaUploadMock.lastAlbumItems) != 3 {
		t.Errorf("expected 3 album items, got %d", len(mediaUploadMock.lastAlbumItems))
	}
	if mediaUploadMock.uploadPhotoCalls != 0 {
		t.Errorf("expected 0 UploadPhoto calls, got %d", mediaUploadMock.uploadPhotoCalls)
	}
	if mediaUploadMock.uploadVideoCalls != 0 {
		t.Errorf("expected 0 UploadVideo calls, got %d", mediaUploadMock.uploadVideoCalls)
	}
}

func TestProcessTaskVideoOnlyRoute(t *testing.T) {
	downloadMock := &mockDownloadRepository{
		validateResult:   true,
		validatePlatform: entity.PlatformInfo{Name: "YouTube"},
		downloadResult:   &entity.DownloadResult{Success: true, FileName: "video.mp4", FileSize: 1024},
		filesToCreate:    []string{"video.mp4"},
	}
	mediaUploadMock := &mockMediaUploadRepository{}

	tasks := []*botEntity.Task{
		{ID: 1, Link: "https://youtube.com/watch?v=abc", GroupIDs: []int64{100}, StatusMessageIDs: map[int64]int{100: 1}},
	}
	taskRepo := &mockTaskRepository{tasks: tasks}
	cfg := newTestEnvWithWorkerCount(1)

	svc := service.NewMediaService(cfg, taskRepo, downloadMock, mediaUploadMock, newTestLogger())
	svc.StartWorkers()
	svc.ScheduleAvailableTasks()

	time.Sleep(500 * time.Millisecond)
	svc.StopWorkers()

	if mediaUploadMock.uploadVideoCalls != 1 {
		t.Errorf("expected 1 UploadVideo call, got %d", mediaUploadMock.uploadVideoCalls)
	}
	if mediaUploadMock.uploadPhotoCalls != 0 {
		t.Errorf("expected 0 UploadPhoto calls, got %d", mediaUploadMock.uploadPhotoCalls)
	}
	if mediaUploadMock.uploadAlbumCalls != 0 {
		t.Errorf("expected 0 UploadAlbum calls, got %d", mediaUploadMock.uploadAlbumCalls)
	}
}

func TestProcessTaskNoFilesFailure(t *testing.T) {
	downloadMock := &mockDownloadRepository{
		validateResult:   true,
		validatePlatform: entity.PlatformInfo{Name: "Test"},
		downloadResult:   &entity.DownloadResult{Success: true},
		// No filesToCreate, no createFileInOutputDir — download produces nothing
	}
	mediaUploadMock := &mockMediaUploadRepository{}

	tasks := []*botEntity.Task{
		{ID: 1, Link: "https://example.com/video", GroupIDs: []int64{100}, StatusMessageIDs: map[int64]int{100: 1}},
	}
	taskRepo := &mockTaskRepository{tasks: tasks}
	cfg := newTestEnvWithWorkerCount(1)

	svc := service.NewMediaService(cfg, taskRepo, downloadMock, mediaUploadMock, newTestLogger())
	svc.StartWorkers()
	svc.ScheduleAvailableTasks()

	time.Sleep(500 * time.Millisecond)
	svc.StopWorkers()

	events := svc.GetMediaEvents()
	select {
	case evt := <-events:
		failure, ok := evt.(entity.MediaProcessFailure)
		if !ok {
			t.Fatalf("expected MediaProcessFailure event, got %T", evt)
		}
		if failure.ErrorMessage == "" {
			t.Error("expected non-empty error message")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("expected a MediaProcessFailure event but none received")
	}
}
