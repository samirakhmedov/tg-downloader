package service

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"tg-downloader/env"
	"tg-downloader/src/core/logger"
	botRepo "tg-downloader/src/features/bot/domain/repository"
	mediaDataRepo "tg-downloader/src/features/media/data/repository"
	"tg-downloader/src/features/media/domain/entity"
	"tg-downloader/src/features/media/domain/repository"
	"time"
)

type MediaTask struct {
	ID               int
	Link             string
	GroupIDs         []int64
	StatusMessageIDs map[int64]int // groupID -> messageID for status messages
}

type MediaService struct {
	environment     env.TGDownloader
	taskRepo        botRepo.ITaskRepository
	downloadRepo    repository.IMediaDownloadRepository
	mediaUploadRepo repository.IMediaUploadRepository
	taskQueue       chan MediaTask
	stopChannel     chan struct{}
	eventChannel    chan entity.MediaEvent
	wg              sync.WaitGroup
	running         bool
	mutex           sync.RWMutex
	logger          *logger.Logger
}

// NewMediaService creates a new MediaService with all required dependencies.
func NewMediaService(
	environment env.TGDownloader,
	taskRepo botRepo.ITaskRepository,
	downloadRepo repository.IMediaDownloadRepository,
	mediaUploadRepo repository.IMediaUploadRepository,
	logger *logger.Logger,
) *MediaService {
	return &MediaService{
		environment:     environment,
		taskRepo:        taskRepo,
		downloadRepo:    downloadRepo,
		mediaUploadRepo: mediaUploadRepo,
		taskQueue:       make(chan MediaTask, environment.WorkerConfiguration.WorkerCount),
		stopChannel:     make(chan struct{}),
		eventChannel:    make(chan entity.MediaEvent, 100),
		running:         false,
		logger:          logger,
	}
}

func (s *MediaService) StartWorkers() {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.running {
		return
	}

	s.running = true

	// Start worker pool
	for i := 0; i < s.environment.WorkerConfiguration.WorkerCount; i++ {
		s.wg.Add(1)
		go s.worker()
	}

	// Start task scheduler
	s.wg.Add(1)
	go s.taskScheduler()

	s.logger.Debug(fmt.Sprintf("MediaService started with %d workers", s.environment.WorkerConfiguration.WorkerCount))
}

func (s *MediaService) StopWorkers() {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if !s.running {
		return
	}

	s.running = false
	close(s.stopChannel)
	s.wg.Wait()

	s.logger.Debug("MediaService stopped")
}

func (s *MediaService) ProcessMedia(link string, groupID int64, messageID int) error {
	_, err := s.taskRepo.CreateTask(link, groupID, messageID)
	return err
}

func (s *MediaService) IsTaskExistsForGroup(link string, groupID int64) bool {
	task, err := s.taskRepo.FindTaskByLink(link)
	if err != nil {
		return false
	}
	for _, gid := range task.GroupIDs {
		if gid == groupID {
			return true
		}
	}
	return false
}

func (s *MediaService) GetMediaEvents() entity.MediaEvents {
	return s.eventChannel
}

func (s *MediaService) taskScheduler() {
	defer s.wg.Done()

	ticker := time.NewTicker(time.Duration(s.environment.WorkerConfiguration.TaskPollingInterval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopChannel:
			return
		case <-ticker.C:
			s.ScheduleAvailableTasks()
		}
	}
}

// ScheduleAvailableTasks fetches pending tasks and queues them for workers.
// It stops after the first blocked send when taskQueue is full (labeled-break).
// Exported so tests can verify the labeled-break behavior directly.
func (s *MediaService) ScheduleAvailableTasks() {
	s.logger.Debug("scheduleAvailableTasks called")

	taskCount := 0
scheduleLoop:
	for {
		s.logger.Debug(fmt.Sprintf("Attempting to get next task (iteration %d)", taskCount))
		task, err := s.taskRepo.GetNextTask()
		if err != nil {
			s.logger.Debug(fmt.Sprintf("No more tasks available after %d tasks, error: %v", taskCount, err))
			break
		}

		taskCount++
		s.logger.Debug(fmt.Sprintf("Found task %d: %s for groups %v", task.ID, task.Link, task.GroupIDs))

		s.logger.Debug(fmt.Sprintf("Marking task %d as in progress", task.ID))
		if err := s.taskRepo.MarkTaskInProgress(task.ID); err != nil {
			s.logger.Debug(fmt.Sprintf("Failed to mark task %d as in progress: %v", task.ID, err))
			continue
		}
		s.logger.Debug(fmt.Sprintf("Successfully marked task %d as in progress", task.ID))

		s.logger.Debug(fmt.Sprintf("Attempting to queue task %d", task.ID))
		select {
		case s.taskQueue <- MediaTask{ID: task.ID, Link: task.Link, GroupIDs: task.GroupIDs, StatusMessageIDs: task.StatusMessageIDs}:
			s.logger.Debug(fmt.Sprintf("Queued task %d for processing in %d groups", task.ID, len(task.GroupIDs)))
		default:
			s.logger.Debug(fmt.Sprintf("Task queue is full, task %d will be processed in next cycle", task.ID))
			break scheduleLoop
		}
	}

	if taskCount == 0 {
		s.logger.Debug("scheduleAvailableTasks completed - no tasks found")
	} else {
		s.logger.Debug(fmt.Sprintf("scheduleAvailableTasks completed - processed %d tasks", taskCount))
	}
}

func (s *MediaService) worker() {
	defer s.wg.Done()

	for {
		select {
		case <-s.stopChannel:
			return
		case task := <-s.taskQueue:
			s.logger.Debug(fmt.Sprintf("Worker processing task %d for groups %v", task.ID, task.GroupIDs))
			s.processTask(task.ID, task.Link, task.GroupIDs, task.StatusMessageIDs)
		}
	}
}

func (s *MediaService) processTask(taskID int, link string, groupIDs []int64, statusMessageIDs map[int64]int) {
	s.logger.Debug(fmt.Sprintf("Starting to process task %d with link: %s for groups: %v", taskID, link, groupIDs))

	isValid, platform, err := s.downloadRepo.ValidateURL(link)
	if err != nil || !isValid {
		s.logger.Debug(fmt.Sprintf("URL validation failed for task %d: %v", taskID, err))
		s.handleTaskFailure(taskID, groupIDs, statusMessageIDs, fmt.Sprintf("Invalid URL: %v", err))
		return
	}

	s.logger.Debug(fmt.Sprintf("Processing %s media: %s", platform.Name, link))

	tempDir, err := os.MkdirTemp("", "tg-dl-*")
	if err != nil {
		s.handleTaskFailure(taskID, groupIDs, statusMessageIDs, "Internal error: could not create temp directory")
		return
	}
	s.logger.Debug(fmt.Sprintf("Task %d temp dir: %s", taskID, tempDir))
	defer os.RemoveAll(tempDir)

	s.logger.Debug(fmt.Sprintf("Starting download for task %d to directory: %s", taskID, tempDir))

	result, err := s.downloadRepo.DownloadMedia(link, tempDir, platform)
	if err != nil || !result.Success {
		s.logger.Debug(fmt.Sprintf("Download failed for task %d: %v", taskID, err))
		if result != nil {
			s.logger.Debug(fmt.Sprintf("Download result error: %v", result.Error))
		}
		s.handleTaskFailure(taskID, groupIDs, statusMessageIDs, classifyDownloadError(result.Error))
		return
	}

	s.logger.Debug(fmt.Sprintf("Download successful for task %d, classifying output dir", taskID))

	items, scanErr := mediaDataRepo.ClassifyOutputDir(tempDir)
	if scanErr != nil || len(items) == 0 {
		s.logger.Debug(fmt.Sprintf("No media files found in output dir for task %d", taskID))
		s.handleTaskFailure(taskID, groupIDs, statusMessageIDs, classifyDownloadError(nil))
		return
	}

	var images []entity.MediaItem
	var videos []entity.MediaItem
	for _, item := range items {
		switch item.Type {
		case entity.MediaTypePhoto:
			images = append(images, item)
		case entity.MediaTypeVideo:
			videos = append(videos, item)
		}
	}

	s.logger.Debug(fmt.Sprintf("Task %d classified: %d images, %d videos", taskID, len(images), len(videos)))

	for _, groupID := range groupIDs {
		messageID := statusMessageIDs[groupID]
		if messageID > 0 {
			s.logger.Debug(fmt.Sprintf("Emitting upload started event for group %d", groupID))
			select {
			case s.eventChannel <- entity.MediaUploadStarted{GroupID: groupID, MessageID: messageID}:
				s.logger.Debug(fmt.Sprintf("Successfully emitted upload started event for group %d", groupID))
			default:
				s.logger.Warn(fmt.Sprintf("Event channel is full, dropping upload started event for group %d", groupID))
			}
		}
	}

	if len(images) == 0 && len(videos) > 0 {
		s.logger.Debug(fmt.Sprintf("Routing task %d to video upload path", taskID))
		for _, groupID := range groupIDs {
			s.logger.Debug(fmt.Sprintf("Uploading video to group %d", groupID))
			if err := s.mediaUploadRepo.UploadVideo(context.Background(), videos[0].FilePath, groupID); err != nil {
				s.logger.Debug(fmt.Sprintf("Failed to upload video to group %d: %v", groupID, err))
			}
		}
	} else if len(images) == 1 && len(videos) == 0 {
		s.logger.Debug(fmt.Sprintf("Routing task %d to single photo upload path", taskID))
		for _, groupID := range groupIDs {
			s.logger.Debug(fmt.Sprintf("Uploading photo to group %d", groupID))
			if err := s.mediaUploadRepo.UploadPhoto(context.Background(), images[0].FilePath, groupID); err != nil {
				s.logger.Debug(fmt.Sprintf("Failed to upload photo to group %d: %v", groupID, err))
			}
		}
	} else {
		s.logger.Debug(fmt.Sprintf("Routing task %d to album upload path (%d items)", taskID, len(items)))
		for _, groupID := range groupIDs {
			s.logger.Debug(fmt.Sprintf("Uploading album to group %d", groupID))
			if err := s.mediaUploadRepo.UploadAlbum(context.Background(), items, groupID); err != nil {
				s.logger.Debug(fmt.Sprintf("Failed to upload album to group %d: %v", groupID, err))
			}
		}
	}

	s.handleTaskSuccess(taskID, groupIDs, statusMessageIDs)
}

func (s *MediaService) handleTaskSuccess(taskID int, groupIDs []int64, statusMessageIDs map[int64]int) {
	s.logger.Debug(fmt.Sprintf("handleTaskSuccess called for task %d, groups %v", taskID, groupIDs))

	if err := s.taskRepo.DeleteTask(taskID); err != nil {
		s.logger.Debug(fmt.Sprintf("Failed to delete completed task %d: %v", taskID, err))
	} else {
		s.logger.Debug(fmt.Sprintf("Successfully deleted completed task %d", taskID))
	}

	for _, groupID := range groupIDs {
		messageID := statusMessageIDs[groupID]
		s.logger.Debug(fmt.Sprintf("Emitting success event for group %d with messageID=%d", groupID, messageID))
		select {
		case s.eventChannel <- entity.MediaProcessSuccess{GroupID: groupID, MessageID: messageID}:
			s.logger.Debug(fmt.Sprintf("Successfully emitted success event for group %d", groupID))
		default:
			s.logger.Warn(fmt.Sprintf("Event channel is full, dropping success event for group %d", groupID))
		}
	}

	s.logger.Debug(fmt.Sprintf("Successfully processed media for groups %v", groupIDs))
}

func (s *MediaService) handleTaskFailure(taskID int, groupIDs []int64, statusMessageIDs map[int64]int, errorMessage string) {
	s.logger.Debug(fmt.Sprintf("handleTaskFailure called for task %d, groups %v, error: %s", taskID, groupIDs, errorMessage))

	if err := s.taskRepo.DeleteTask(taskID); err != nil {
		s.logger.Debug(fmt.Sprintf("Failed to delete failed task %d: %v", taskID, err))
	} else {
		s.logger.Debug(fmt.Sprintf("Successfully deleted failed task %d", taskID))
	}

	for _, groupID := range groupIDs {
		messageID := statusMessageIDs[groupID]
		s.logger.Debug(fmt.Sprintf("Emitting failure event for group %d with messageID=%d, error=%s", groupID, messageID, errorMessage))
		select {
		case s.eventChannel <- entity.MediaProcessFailure{GroupID: groupID, MessageID: messageID, ErrorMessage: errorMessage}:
			s.logger.Debug(fmt.Sprintf("Successfully emitted failure event for group %d", groupID))
		default:
			s.logger.Warn(fmt.Sprintf("Event channel is full, dropping failure event for group %d", groupID))
		}
	}

	s.logger.Debug(fmt.Sprintf("Failed to process media for groups %v: %s", groupIDs, errorMessage))
}

// ClassifyDownloadError translates yt-dlp error strings into user-facing messages.
// Exported so tests can verify each error classification branch directly.
func ClassifyDownloadError(err error) string {
	if err == nil {
		return "Download failed. Please try again later."
	}
	msg := err.Error()
	switch {
	case strings.Contains(msg, "context deadline exceeded"):
		return "Download timed out. Please try again later."
	case strings.Contains(msg, "Private video"),
		strings.Contains(msg, "This video is private"):
		return "This video is private and cannot be downloaded."
	case strings.Contains(msg, "Unsupported URL"),
		strings.Contains(msg, "is not a supported URL"):
		return "This URL is not supported."
	case strings.Contains(msg, "Failed to resolve"),
		strings.Contains(msg, "Temporary failure in name resolution"),
		strings.Contains(msg, "Unable to download webpage"):
		return "Download failed due to a network error. Please try again."
	default:
		return "Download failed. Please try again later."
	}
}

var classifyDownloadError = ClassifyDownloadError
