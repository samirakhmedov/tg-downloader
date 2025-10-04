package service

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"tg-downloader/env"
	"tg-downloader/src/core"
	"tg-downloader/src/core/logger"
	botRepo "tg-downloader/src/features/bot/domain/repository"
	"tg-downloader/src/features/media/domain/entity"
	"tg-downloader/src/features/media/domain/repository"
	"time"
)

type MediaTask struct {
	ID       int
	Link     string
	GroupIDs []int64
}

type MediaService struct {
	environment  env.TGDownloader
	taskRepo     botRepo.ITaskRepository
	downloadRepo repository.IDownloadRepository
	uploadRepo   repository.IUploadRepository
	taskQueue    chan MediaTask
	stopChannel  chan struct{}
	eventChannel chan entity.MediaEvent
	wg           sync.WaitGroup
	running      bool
	mutex        sync.RWMutex
	logger       *logger.Logger
}

// NewMediaService creates a new MediaService with all required dependencies.
// The logger parameter allows dynamic control of logging behavior throughout media processing.
func NewMediaService(
	environment env.TGDownloader,
	taskRepo botRepo.ITaskRepository,
	downloadRepo repository.IDownloadRepository,
	uploadRepo repository.IUploadRepository,
	logger *logger.Logger,
) *MediaService {
	return &MediaService{
		environment:  environment,
		taskRepo:     taskRepo,
		downloadRepo: downloadRepo,
		uploadRepo:   uploadRepo,
		taskQueue:    make(chan MediaTask, environment.WorkerConfiguration.WorkerCount),
		stopChannel:  make(chan struct{}),
		eventChannel: make(chan entity.MediaEvent, 100),
		running:      false,
		logger:       logger,
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

func (s *MediaService) ProcessMedia(link string, groupID int64) error {
	_, err := s.taskRepo.CreateTask(link, groupID)
	return err
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
			// Wake up all idle workers to check for tasks
			s.scheduleAvailableTasks()
		}
	}
}

func (s *MediaService) scheduleAvailableTasks() {
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
		case s.taskQueue <- MediaTask{ID: task.ID, Link: task.Link, GroupIDs: task.GroupIDs}:
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

func (s *MediaService) worker() {
	defer s.wg.Done()

	for {
		select {
		case <-s.stopChannel:
			return
		case task := <-s.taskQueue:
			// Process the task
			s.logger.Debug(fmt.Sprintf("Worker processing task %d for groups %v", task.ID, task.GroupIDs))
			s.processTask(task.ID, task.Link, task.GroupIDs)
		}
	}
}

func (s *MediaService) processTask(taskID int, link string, groupIDs []int64) {
	s.logger.Debug(fmt.Sprintf("Starting to process task %d with link: %s for groups: %v", taskID, link, groupIDs))

	_, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Validate URL first
	isValid, platformName, err := s.downloadRepo.ValidateURL(link)
	if err != nil || !isValid {
		s.logger.Debug(fmt.Sprintf("URL validation failed for task %d: %v", taskID, err))
		s.handleTaskFailure(taskID, groupIDs, fmt.Sprintf("Invalid URL: %v", err))
		return
	}

	s.logger.Debug(fmt.Sprintf("Processing %s media: %s", platformName, link))

	// Download media to a shared directory (use first group ID for directory)
	outputDir := fmt.Sprintf("%s/shared", core.MediaOutputDirectory)
	s.logger.Debug(fmt.Sprintf("Starting download for task %d to directory: %s", taskID, outputDir))

	result, err := s.downloadRepo.Download(link, outputDir)
	if err != nil || !result.Success {
		s.logger.Debug(fmt.Sprintf("Download failed for task %d: %v", taskID, err))
		if result != nil {
			s.logger.Debug(fmt.Sprintf("Download result error: %v", result.Error))
		}
		s.handleTaskFailure(taskID, groupIDs, fmt.Sprintf("Download failed: %v", result.Error))
		return
	}

	s.logger.Debug(fmt.Sprintf("Download successful for task %d, files: %d", taskID, len(result.Files)))

	// Upload to all groups
	uploadCount := 0
	for _, groupID := range groupIDs {
		s.logger.Debug(fmt.Sprintf("Uploading to group %d", groupID))
		err = s.uploadRepo.UploadMedia(result.Files, groupID)
		if err != nil {
			s.logger.Debug(fmt.Sprintf("Failed to upload to group %d: %v", groupID, err))
			// Continue uploading to other groups
		} else {
			uploadCount++
			s.logger.Debug(fmt.Sprintf("Successfully uploaded to group %d", groupID))
		}
	}

	// Success - clean up and notify all groups
	for _, file := range result.Files {
		s.logger.Debug(fmt.Sprintf("Cleaning up file: %s", file.FilePath))
		os.Remove(file.FilePath)
	}

	// Extract filenames for success event
	var fileNames []string
	for _, file := range result.Files {
		fileNames = append(fileNames, file.FileName)
	}

	s.logger.Debug(fmt.Sprintf("Calling success handler for task %d with %d successful uploads", taskID, uploadCount))
	s.handleTaskSuccess(taskID, groupIDs, fileNames)
}

func (s *MediaService) handleTaskSuccess(taskID int, groupIDs []int64, fileNames []string) {
	s.logger.Debug(fmt.Sprintf("handleTaskSuccess called for task %d, groups %v, files %v", taskID, groupIDs, fileNames))

	// Delete completed task
	if err := s.taskRepo.DeleteTask(taskID); err != nil {
		s.logger.Debug(fmt.Sprintf("Failed to delete completed task %d: %v", taskID, err))
	} else {
		s.logger.Debug(fmt.Sprintf("Successfully deleted completed task %d", taskID))
	}

	// Create display string for files
	displayNames := strings.Join(fileNames, ", ")

	// Emit success events for all groups
	for _, groupID := range groupIDs {
		s.logger.Debug(fmt.Sprintf("Emitting success event for group %d with fileNames=%v", groupID, fileNames))
		select {
		case s.eventChannel <- entity.MediaProcessSuccess{GroupID: groupID, FileNames: fileNames}:
			s.logger.Debug(fmt.Sprintf("Successfully emitted success event for group %d", groupID))
		default:
			s.logger.Warn(fmt.Sprintf("Event channel is full, dropping success event for group %d", groupID))
		}
	}

	s.logger.Debug(fmt.Sprintf("Successfully processed media for groups %v: %s", groupIDs, displayNames))
}

func (s *MediaService) handleTaskFailure(taskID int, groupIDs []int64, errorMessage string) {
	s.logger.Debug(fmt.Sprintf("handleTaskFailure called for task %d, groups %v, error: %s", taskID, groupIDs, errorMessage))

	// Delete failed task
	if err := s.taskRepo.DeleteTask(taskID); err != nil {
		s.logger.Debug(fmt.Sprintf("Failed to delete failed task %d: %v", taskID, err))
	} else {
		s.logger.Debug(fmt.Sprintf("Successfully deleted failed task %d", taskID))
	}

	// Emit failure events for all groups
	for _, groupID := range groupIDs {
		s.logger.Debug(fmt.Sprintf("Emitting failure event for group %d with error=%s", groupID, errorMessage))
		select {
		case s.eventChannel <- entity.MediaProcessFailure{GroupID: groupID, ErrorMessage: errorMessage}:
			s.logger.Debug(fmt.Sprintf("Successfully emitted failure event for group %d", groupID))
		default:
			s.logger.Warn(fmt.Sprintf("Event channel is full, dropping failure event for group %d", groupID))
		}
	}

	s.logger.Debug(fmt.Sprintf("Failed to process media for groups %v: %s", groupIDs, errorMessage))
}
