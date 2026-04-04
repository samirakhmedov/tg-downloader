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
	mediaEntity "tg-downloader/src/features/media/domain/entity"
	iMediaRepo "tg-downloader/src/features/media/domain/repository"
	"tg-downloader/src/features/video/domain/entity"
	videoRepo "tg-downloader/src/features/video/domain/repository"
	"tg-downloader/src/features/video/domain/service"
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

// mockDownloadRepository is a configurable IVideoDownloadRepository stub.
type mockDownloadRepository struct {
	validateResult        bool
	validatePlatform      entity.PlatformInfo
	validateErr           error
	downloadResult        *entity.VideoProcessResult
	downloadErr           error
	createFileInOutputDir bool
	filesToCreate         []string // list of filenames to create (e.g., "image.jpg", "clip.mp4")
}

func (m *mockDownloadRepository) ValidateURL(url string) (bool, entity.PlatformInfo, error) {
	return m.validateResult, m.validatePlatform, m.validateErr
}

func (m *mockDownloadRepository) DownloadVideo(url string, outputDir string, platform entity.PlatformInfo) (*entity.VideoProcessResult, error) {
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

// Ensure mockDownloadRepository implements IVideoDownloadRepository.
var _ videoRepo.IVideoDownloadRepository = (*mockDownloadRepository)(nil)

// mockUploadRepository is a configurable IUploadRepository stub.
type mockUploadRepository struct {
	err              error
	uploadVideoCalls int
}

func (m *mockUploadRepository) UploadVideo(ctx context.Context, filePath string, groupID int64) error {
	m.uploadVideoCalls++
	return m.err
}

// Ensure mockUploadRepository implements IUploadRepository.
var _ videoRepo.IUploadRepository = (*mockUploadRepository)(nil)

// mockMediaUploadRepository is a configurable IMediaUploadRepository stub.
type mockMediaUploadRepository struct {
	uploadPhotoCalls int
	uploadAlbumCalls int
	lastAlbumItems   []mediaEntity.MediaItem
	photoErr         error
	albumErr         error
}

func (m *mockMediaUploadRepository) UploadPhoto(ctx context.Context, filePath string, groupID int64) error {
	m.uploadPhotoCalls++
	return m.photoErr
}

func (m *mockMediaUploadRepository) UploadAlbum(ctx context.Context, items []mediaEntity.MediaItem, groupID int64) error {
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
// Verifies the labeled-break fix (BUGF-01):
// When taskQueue capacity is 1 and 3 tasks are available, ScheduleAvailableTasks
// marks a task in_progress before attempting to queue it. With capacity=1:
//   - Task 1 is marked in_progress, then queued successfully (queue is now full).
//   - Task 2 is marked in_progress, then the queue send blocks → labeled-break exits.
//   - Task 3 is never fetched or marked (loop stopped after the first blocked send).
//
// Without the labeled-break fix the loop would continue to task 3, marking it in_progress
// even though the queue is full — causing it to be lost until the next scheduler cycle.
func TestScheduleStopsWhenQueueFull(t *testing.T) {
	tasks := []*botEntity.Task{
		{ID: 1, Link: "https://example.com/1", GroupIDs: []int64{100}, StatusMessageIDs: map[int64]int{100: 1}},
		{ID: 2, Link: "https://example.com/2", GroupIDs: []int64{100}, StatusMessageIDs: map[int64]int{100: 2}},
		{ID: 3, Link: "https://example.com/3", GroupIDs: []int64{100}, StatusMessageIDs: map[int64]int{100: 3}},
	}

	taskRepo := &mockTaskRepository{tasks: tasks}

	// WorkerCount=1 means taskQueue capacity=1
	cfg := newTestEnvWithWorkerCount(1)

	svc := service.NewVideoService(
		cfg,
		taskRepo,
		&mockDownloadRepository{},
		&mockUploadRepository{},
		&mockMediaUploadRepository{},
		newTestLogger(),
	)

	// Call ScheduleAvailableTasks directly (no goroutines running).
	// Expected flow:
	//   iteration 1: GetNextTask→task1, MarkInProgress(1), queue←task1 (succeeds, queue full)
	//   iteration 2: GetNextTask→task2, MarkInProgress(2), queue←task2 (fails, queue full) → break
	//   iteration 3: never reached — the labeled-break exits the loop
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

// TestProcessTaskTempDirCleanup verifies that a successful processTask
// removes its per-task temp directory after completion.
func TestProcessTaskTempDirCleanup(t *testing.T) {
	downloadMock := &mockDownloadRepository{
		validateResult:        true,
		validatePlatform:      entity.PlatformInfo{Name: "Test"},
		downloadResult:        &entity.VideoProcessResult{Success: true, FileName: "test.mp4", FileSize: 1024},
		createFileInOutputDir: true,
	}
	uploadMock := &mockUploadRepository{err: nil}

	tasks := []*botEntity.Task{
		{ID: 1, Link: "https://example.com/video", GroupIDs: []int64{100}, StatusMessageIDs: map[int64]int{100: 1}},
	}
	taskRepo := &mockTaskRepository{tasks: tasks}
	cfg := newTestEnvWithWorkerCount(1)

	svc := service.NewVideoService(cfg, taskRepo, downloadMock, uploadMock, &mockMediaUploadRepository{}, newTestLogger())
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

// TestProcessTaskTempDirCleanedOnDownloadFailure verifies that even when download
// fails, the per-task temp directory is cleaned up.
func TestProcessTaskTempDirCleanedOnDownloadFailure(t *testing.T) {
	downloadMock := &mockDownloadRepository{
		validateResult:   true,
		validatePlatform: entity.PlatformInfo{Name: "Test"},
		downloadResult:   &entity.VideoProcessResult{Success: false, Error: errors.New("download failed")},
		downloadErr:      errors.New("download failed"),
	}
	uploadMock := &mockUploadRepository{}

	tasks := []*botEntity.Task{
		{ID: 1, Link: "https://example.com/video", GroupIDs: []int64{100}, StatusMessageIDs: map[int64]int{100: 1}},
	}
	taskRepo := &mockTaskRepository{tasks: tasks}
	cfg := newTestEnvWithWorkerCount(1)

	svc := service.NewVideoService(cfg, taskRepo, downloadMock, uploadMock, &mockMediaUploadRepository{}, newTestLogger())
	svc.StartWorkers()
	svc.ScheduleAvailableTasks()

	time.Sleep(500 * time.Millisecond)
	svc.StopWorkers()

	matches, _ := filepath.Glob(filepath.Join(os.TempDir(), "tg-dl-*"))
	if len(matches) > 0 {
		t.Errorf("expected no tg-dl-* temp directories after download failure, found: %v", matches)
	}
}

// TestUploadFailureLeavesNoTempFiles verifies that when upload fails, the per-task
// temp directory is still cleaned up (not left behind as orphaned files).
func TestUploadFailureLeavesNoTempFiles(t *testing.T) {
	downloadMock := &mockDownloadRepository{
		validateResult:        true,
		validatePlatform:      entity.PlatformInfo{Name: "Test"},
		downloadResult:        &entity.VideoProcessResult{Success: true, FileName: "test.mp4", FileSize: 1024},
		createFileInOutputDir: true,
	}
	uploadMock := &mockUploadRepository{err: errors.New("upload failed: connection timeout")}

	tasks := []*botEntity.Task{
		{ID: 1, Link: "https://example.com/video", GroupIDs: []int64{100}, StatusMessageIDs: map[int64]int{100: 1}},
	}
	taskRepo := &mockTaskRepository{tasks: tasks}
	cfg := newTestEnvWithWorkerCount(1)

	svc := service.NewVideoService(cfg, taskRepo, downloadMock, uploadMock, &mockMediaUploadRepository{}, newTestLogger())
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

// TestProcessTaskSingleImageRoute verifies that a single image file routes to UploadPhoto,
// not UploadVideo or UploadAlbum.
func TestProcessTaskSingleImageRoute(t *testing.T) {
	downloadMock := &mockDownloadRepository{
		validateResult:   true,
		validatePlatform: entity.PlatformInfo{Name: "Instagram Posts"},
		downloadResult:   &entity.VideoProcessResult{Success: true, FileName: "photo.jpg", FileSize: 1024},
		filesToCreate:    []string{"photo.jpg"},
	}
	uploadMock := &mockUploadRepository{}
	mediaUploadMock := &mockMediaUploadRepository{}

	tasks := []*botEntity.Task{
		{ID: 1, Link: "https://instagram.com/p/abc123", GroupIDs: []int64{100}, StatusMessageIDs: map[int64]int{100: 1}},
	}
	taskRepo := &mockTaskRepository{tasks: tasks}
	cfg := newTestEnvWithWorkerCount(1)

	svc := service.NewVideoService(cfg, taskRepo, downloadMock, uploadMock, mediaUploadMock, newTestLogger())
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
	if uploadMock.uploadVideoCalls != 0 {
		t.Errorf("expected 0 UploadVideo calls, got %d", uploadMock.uploadVideoCalls)
	}
}

// TestProcessTaskMultiImageRoute verifies that multiple image files route to UploadAlbum.
func TestProcessTaskMultiImageRoute(t *testing.T) {
	downloadMock := &mockDownloadRepository{
		validateResult:   true,
		validatePlatform: entity.PlatformInfo{Name: "Instagram Posts"},
		downloadResult:   &entity.VideoProcessResult{Success: true, FileName: "slide1.jpg", FileSize: 1024},
		filesToCreate:    []string{"slide1.jpg", "slide2.png", "slide3.webp"},
	}
	uploadMock := &mockUploadRepository{}
	mediaUploadMock := &mockMediaUploadRepository{}

	tasks := []*botEntity.Task{
		{ID: 1, Link: "https://instagram.com/p/abc123", GroupIDs: []int64{100}, StatusMessageIDs: map[int64]int{100: 1}},
	}
	taskRepo := &mockTaskRepository{tasks: tasks}
	cfg := newTestEnvWithWorkerCount(1)

	svc := service.NewVideoService(cfg, taskRepo, downloadMock, uploadMock, mediaUploadMock, newTestLogger())
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
	if uploadMock.uploadVideoCalls != 0 {
		t.Errorf("expected 0 UploadVideo calls, got %d", uploadMock.uploadVideoCalls)
	}
}

// TestProcessTaskMixedMediaRoute verifies that mixed photo+video files route to UploadAlbum.
func TestProcessTaskMixedMediaRoute(t *testing.T) {
	downloadMock := &mockDownloadRepository{
		validateResult:   true,
		validatePlatform: entity.PlatformInfo{Name: "Instagram Posts"},
		downloadResult:   &entity.VideoProcessResult{Success: true, FileName: "photo1.jpg", FileSize: 1024},
		filesToCreate:    []string{"photo1.jpg", "clip.mp4", "photo2.webp"},
	}
	uploadMock := &mockUploadRepository{}
	mediaUploadMock := &mockMediaUploadRepository{}

	tasks := []*botEntity.Task{
		{ID: 1, Link: "https://instagram.com/p/abc123", GroupIDs: []int64{100}, StatusMessageIDs: map[int64]int{100: 1}},
	}
	taskRepo := &mockTaskRepository{tasks: tasks}
	cfg := newTestEnvWithWorkerCount(1)

	svc := service.NewVideoService(cfg, taskRepo, downloadMock, uploadMock, mediaUploadMock, newTestLogger())
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
	if uploadMock.uploadVideoCalls != 0 {
		t.Errorf("expected 0 UploadVideo calls, got %d", uploadMock.uploadVideoCalls)
	}
}

// TestProcessTaskVideoOnlyRoute verifies that a single video file routes to UploadVideo
// (regression test — existing video path must not break).
func TestProcessTaskVideoOnlyRoute(t *testing.T) {
	downloadMock := &mockDownloadRepository{
		validateResult:   true,
		validatePlatform: entity.PlatformInfo{Name: "YouTube"},
		downloadResult:   &entity.VideoProcessResult{Success: true, FileName: "video.mp4", FileSize: 1024},
		filesToCreate:    []string{"video.mp4"},
	}
	uploadMock := &mockUploadRepository{}
	mediaUploadMock := &mockMediaUploadRepository{}

	tasks := []*botEntity.Task{
		{ID: 1, Link: "https://youtube.com/watch?v=abc", GroupIDs: []int64{100}, StatusMessageIDs: map[int64]int{100: 1}},
	}
	taskRepo := &mockTaskRepository{tasks: tasks}
	cfg := newTestEnvWithWorkerCount(1)

	svc := service.NewVideoService(cfg, taskRepo, downloadMock, uploadMock, mediaUploadMock, newTestLogger())
	svc.StartWorkers()
	svc.ScheduleAvailableTasks()

	time.Sleep(500 * time.Millisecond)
	svc.StopWorkers()

	if uploadMock.uploadVideoCalls != 1 {
		t.Errorf("expected 1 UploadVideo call, got %d", uploadMock.uploadVideoCalls)
	}
	if mediaUploadMock.uploadPhotoCalls != 0 {
		t.Errorf("expected 0 UploadPhoto calls, got %d", mediaUploadMock.uploadPhotoCalls)
	}
	if mediaUploadMock.uploadAlbumCalls != 0 {
		t.Errorf("expected 0 UploadAlbum calls, got %d", mediaUploadMock.uploadAlbumCalls)
	}
}

// TestProcessTaskNoFilesFailure verifies that when download succeeds but produces no
// recognizable media files, a VideoProcessFailure event is emitted.
func TestProcessTaskNoFilesFailure(t *testing.T) {
	downloadMock := &mockDownloadRepository{
		validateResult:   true,
		validatePlatform: entity.PlatformInfo{Name: "Test"},
		downloadResult:   &entity.VideoProcessResult{Success: true},
		// No filesToCreate, no createFileInOutputDir — download produces nothing
	}
	uploadMock := &mockUploadRepository{}
	mediaUploadMock := &mockMediaUploadRepository{}

	tasks := []*botEntity.Task{
		{ID: 1, Link: "https://example.com/video", GroupIDs: []int64{100}, StatusMessageIDs: map[int64]int{100: 1}},
	}
	taskRepo := &mockTaskRepository{tasks: tasks}
	cfg := newTestEnvWithWorkerCount(1)

	svc := service.NewVideoService(cfg, taskRepo, downloadMock, uploadMock, mediaUploadMock, newTestLogger())
	svc.StartWorkers()
	svc.ScheduleAvailableTasks()

	time.Sleep(500 * time.Millisecond)
	svc.StopWorkers()

	events := svc.GetVideoEvents()
	select {
	case evt := <-events:
		failure, ok := evt.(entity.VideoProcessFailure)
		if !ok {
			t.Fatalf("expected VideoProcessFailure event, got %T", evt)
		}
		if failure.ErrorMessage == "" {
			t.Error("expected non-empty error message")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("expected a VideoProcessFailure event but none received")
	}
}
