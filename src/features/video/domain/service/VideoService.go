package service

import (
	"context"
	"fmt"
	"os"
	"sync"
	"tg-downloader/env"
	"tg-downloader/src/core"
	"tg-downloader/src/core/logger"
	botRepo "tg-downloader/src/features/bot/domain/repository"
	"tg-downloader/src/features/video/domain/entity"
	"tg-downloader/src/features/video/domain/repository"
	"time"
)

type VideoTask struct {
	ID               int
	Link             string
	GroupIDs         []int64
	StatusMessageIDs map[int64]int // groupID -> messageID for status messages
}

type VideoService struct {
	environment  env.TGDownloader
	taskRepo     botRepo.ITaskRepository
	downloadRepo repository.IVideoDownloadRepository
	uploadRepo   repository.IUploadRepository
	taskQueue    chan VideoTask
	stopChannel  chan struct{}
	eventChannel chan entity.VideoEvent
	wg           sync.WaitGroup
	running      bool
	mutex        sync.RWMutex
	logger       *logger.Logger
}

// NewVideoService creates a new VideoService with all required dependencies.
// The logger parameter allows dynamic control of logging behavior throughout video processing.
func NewVideoService(
	environment env.TGDownloader,
	taskRepo botRepo.ITaskRepository,
	downloadRepo repository.IVideoDownloadRepository,
	uploadRepo repository.IUploadRepository,
	logger *logger.Logger,
) *VideoService {
	return &VideoService{
		environment:  environment,
		taskRepo:     taskRepo,
		downloadRepo: downloadRepo,
		uploadRepo:   uploadRepo,
		taskQueue:    make(chan VideoTask, environment.WorkerConfiguration.WorkerCount),
		stopChannel:  make(chan struct{}),
		eventChannel: make(chan entity.VideoEvent, 100),
		running:      false,
		logger:       logger,
	}
}

func (s *VideoService) StartWorkers() {
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

	s.logger.Debug(fmt.Sprintf("VideoService started with %d workers", s.environment.WorkerConfiguration.WorkerCount))
}

func (s *VideoService) StopWorkers() {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if !s.running {
		return
	}

	s.running = false
	close(s.stopChannel)
	s.wg.Wait()

	s.logger.Debug("VideoService stopped")
}

func (s *VideoService) ProcessVideo(link string, groupID int64, messageID int) error {
	_, err := s.taskRepo.CreateTask(link, groupID, messageID)
	return err
}

func (s *VideoService) GetVideoEvents() entity.VideoEvents {
	return s.eventChannel
}

func (s *VideoService) taskScheduler() {
	defer s.wg.Done()

	ticker := time.NewTicker(time.Duration(s.environment.WorkerConfiguration.TaskPollingInterval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopChannel:
			return
		case <-ticker.C:
			// Wake up all idle workers to check for tasks
			s.scheduleAvailableTasks()
		}
	}
}

func (s *VideoService) scheduleAvailableTasks() {
	s.logger.Debug("scheduleAvailableTasks called")

	// Get all available tasks and queue them for workers
	taskCount := 0
	for {
		s.logger.Debug(fmt.Sprintf("Attempting to get next task (iteration %d)", taskCount))
		task, err := s.taskRepo.GetNextTask()
		if err != nil {
			// No more tasks available
			s.logger.Debug(fmt.Sprintf("No more tasks available after %d tasks, error: %v", taskCount, err))
			break
		}

		taskCount++
		s.logger.Debug(fmt.Sprintf("Found task %d: %s for groups %v", task.ID, task.Link, task.GroupIDs))

		// Mark task as in progress immediately to prevent duplicate processing
		s.logger.Debug(fmt.Sprintf("Marking task %d as in progress", task.ID))
		if err := s.taskRepo.MarkTaskInProgress(task.ID); err != nil {
			s.logger.Debug(fmt.Sprintf("Failed to mark task %d as in progress: %v", task.ID, err))
			continue
		}
		s.logger.Debug(fmt.Sprintf("Successfully marked task %d as in progress", task.ID))

		// Try to queue the task (non-blocking)
		s.logger.Debug(fmt.Sprintf("Attempting to queue task %d", task.ID))
		select {
		case s.taskQueue <- VideoTask{ID: task.ID, Link: task.Link, GroupIDs: task.GroupIDs, StatusMessageIDs: task.StatusMessageIDs}:
			// Task queued successfully
			s.logger.Debug(fmt.Sprintf("Queued task %d for processing in %d groups", task.ID, len(task.GroupIDs)))
		default:
			// Queue is full, task will be picked up in next cycle
			s.logger.Debug(fmt.Sprintf("Task queue is full, task %d will be processed in next cycle", task.ID))
			break
		}
	}

	if taskCount == 0 {
		s.logger.Debug("scheduleAvailableTasks completed - no tasks found")
	} else {
		s.logger.Debug(fmt.Sprintf("scheduleAvailableTasks completed - processed %d tasks", taskCount))
	}
}

func (s *VideoService) worker() {
	defer s.wg.Done()

	for {
		select {
		case <-s.stopChannel:
			return
		case task := <-s.taskQueue:
			// Process the task
			s.logger.Debug(fmt.Sprintf("Worker processing task %d for groups %v", task.ID, task.GroupIDs))
			s.processTask(task.ID, task.Link, task.GroupIDs, task.StatusMessageIDs)
		}
	}
}

func (s *VideoService) processTask(taskID int, link string, groupIDs []int64, statusMessageIDs map[int64]int) {
	s.logger.Debug(fmt.Sprintf("Starting to process task %d with link: %s for groups: %v", taskID, link, groupIDs))

	_, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Validate URL first
	isValid, platformName, err := s.downloadRepo.ValidateURL(link)
	if err != nil || !isValid {
		s.logger.Debug(fmt.Sprintf("URL validation failed for task %d: %v", taskID, err))
		s.handleTaskFailure(taskID, groupIDs, statusMessageIDs, fmt.Sprintf("Invalid URL: %v", err))
		return
	}

	s.logger.Debug(fmt.Sprintf("Processing %s video: %s", platformName, link))

	// Download video to a shared directory (use first group ID for directory)
	outputDir := fmt.Sprintf("%s/shared", core.VideoOutputDirectory)
	s.logger.Debug(fmt.Sprintf("Starting download for task %d to directory: %s", taskID, outputDir))

	result, err := s.downloadRepo.DownloadVideo(link, outputDir)
	if err != nil || !result.Success {
		s.logger.Debug(fmt.Sprintf("Download failed for task %d: %v", taskID, err))
		if result != nil {
			s.logger.Debug(fmt.Sprintf("Download result error: %v", result.Error))
		}
		s.handleTaskFailure(taskID, groupIDs, statusMessageIDs, fmt.Sprintf("Download failed: %v", result.Error))
		return
	}

	s.logger.Debug(fmt.Sprintf("Download successful for task %d, file: %s", taskID, result.FilePath))

	// Emit upload started events for all groups
	for _, groupID := range groupIDs {
		messageID := statusMessageIDs[groupID]
		if messageID > 0 {
			s.logger.Debug(fmt.Sprintf("Emitting upload started event for group %d", groupID))
			select {
			case s.eventChannel <- entity.VideoUploadStarted{GroupID: groupID, MessageID: messageID}:
				s.logger.Debug(fmt.Sprintf("Successfully emitted upload started event for group %d", groupID))
			default:
				s.logger.Warn(fmt.Sprintf("Event channel is full, dropping upload started event for group %d", groupID))
			}
		}
	}

	// Upload to all groups
	uploadCount := 0
	for _, groupID := range groupIDs {
		s.logger.Debug(fmt.Sprintf("Uploading to group %d", groupID))
		err = s.uploadRepo.UploadVideo(result.FilePath, groupID)
		if err != nil {
			s.logger.Debug(fmt.Sprintf("Failed to upload to group %d: %v", groupID, err))
			// Continue uploading to other groups
		} else {
			uploadCount++
			s.logger.Debug(fmt.Sprintf("Successfully uploaded to group %d", groupID))
		}
	}

	// Success - clean up and notify all groups
	s.logger.Debug(fmt.Sprintf("Cleaning up file: %s", result.FilePath))
	os.Remove(result.FilePath)

	s.logger.Debug(fmt.Sprintf("Calling success handler for task %d with %d successful uploads", taskID, uploadCount))
	s.handleTaskSuccess(taskID, groupIDs, statusMessageIDs)
}

func (s *VideoService) handleTaskSuccess(taskID int, groupIDs []int64, statusMessageIDs map[int64]int) {
	s.logger.Debug(fmt.Sprintf("handleTaskSuccess called for task %d, groups %v", taskID, groupIDs))

	// Delete completed task
	if err := s.taskRepo.DeleteTask(taskID); err != nil {
		s.logger.Debug(fmt.Sprintf("Failed to delete completed task %d: %v", taskID, err))
	} else {
		s.logger.Debug(fmt.Sprintf("Successfully deleted completed task %d", taskID))
	}

	// Emit success events for all groups
	for _, groupID := range groupIDs {
		messageID := statusMessageIDs[groupID]
		s.logger.Debug(fmt.Sprintf("Emitting success event for group %d with messageID=%d", groupID, messageID))
		select {
		case s.eventChannel <- entity.VideoProcessSuccess{GroupID: groupID, MessageID: messageID}:
			s.logger.Debug(fmt.Sprintf("Successfully emitted success event for group %d", groupID))
		default:
			s.logger.Warn(fmt.Sprintf("Event channel is full, dropping success event for group %d", groupID))
		}
	}

	s.logger.Debug(fmt.Sprintf("Successfully processed video for groups %v", groupIDs))
}

func (s *VideoService) handleTaskFailure(taskID int, groupIDs []int64, statusMessageIDs map[int64]int, errorMessage string) {
	s.logger.Debug(fmt.Sprintf("handleTaskFailure called for task %d, groups %v, error: %s", taskID, groupIDs, errorMessage))

	// Delete failed task
	if err := s.taskRepo.DeleteTask(taskID); err != nil {
		s.logger.Debug(fmt.Sprintf("Failed to delete failed task %d: %v", taskID, err))
	} else {
		s.logger.Debug(fmt.Sprintf("Successfully deleted failed task %d", taskID))
	}

	// Emit failure events for all groups
	for _, groupID := range groupIDs {
		messageID := statusMessageIDs[groupID]
		s.logger.Debug(fmt.Sprintf("Emitting failure event for group %d with messageID=%d, error=%s", groupID, messageID, errorMessage))
		select {
		case s.eventChannel <- entity.VideoProcessFailure{GroupID: groupID, MessageID: messageID, ErrorMessage: errorMessage}:
			s.logger.Debug(fmt.Sprintf("Successfully emitted failure event for group %d", groupID))
		default:
			s.logger.Warn(fmt.Sprintf("Event channel is full, dropping failure event for group %d", groupID))
		}
	}

	s.logger.Debug(fmt.Sprintf("Failed to process video for groups %v: %s", groupIDs, errorMessage))
}
