package service

import (
	"context"
	"fmt"
	"log"
	"os"
	"sync"
	"tg-downloader/env"
	"tg-downloader/src/core"
	botRepo "tg-downloader/src/features/bot/domain/repository"
	"tg-downloader/src/features/video/domain/entity"
	"tg-downloader/src/features/video/domain/repository"
	"time"
)

type VideoTask struct {
	ID       int
	Link     string
	GroupIDs []int64
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
}

func NewVideoService(
	environment env.TGDownloader,
	taskRepo botRepo.ITaskRepository,
	downloadRepo repository.IVideoDownloadRepository,
	uploadRepo repository.IUploadRepository,
) *VideoService {
	return &VideoService{
		environment:  environment,
		taskRepo:     taskRepo,
		downloadRepo: downloadRepo,
		uploadRepo:   uploadRepo,
		taskQueue:    make(chan VideoTask, environment.VideoProcessingConfiguration.WorkerCount),
		stopChannel:  make(chan struct{}),
		eventChannel: make(chan entity.VideoEvent, 100),
		running:      false,
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
	for i := 0; i < s.environment.VideoProcessingConfiguration.WorkerCount; i++ {
		s.wg.Add(1)
		go s.worker()
	}

	// Start task scheduler
	s.wg.Add(1)
	go s.taskScheduler()

	log.Printf("VideoService started with %d workers", s.environment.VideoProcessingConfiguration.WorkerCount)
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

	log.Println("VideoService stopped")
}

func (s *VideoService) ProcessVideo(link string, groupID int64) error {
	_, err := s.taskRepo.CreateTask(link, groupID)
	return err
}

func (s *VideoService) GetVideoEvents() entity.VideoEvents {
	return s.eventChannel
}

func (s *VideoService) taskScheduler() {
	defer s.wg.Done()

	ticker := time.NewTicker(time.Duration(s.environment.VideoProcessingConfiguration.TaskPollingInterval) * time.Second)
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
	log.Printf("scheduleAvailableTasks called")

	// Get all available tasks and queue them for workers
	taskCount := 0
	for {
		log.Printf("Attempting to get next task (iteration %d)", taskCount)
		task, err := s.taskRepo.GetNextTask()
		if err != nil {
			// No more tasks available
			log.Printf("No more tasks available after %d tasks, error: %v", taskCount, err)
			break
		}

		taskCount++
		log.Printf("Found task %d: %s for groups %v", task.ID, task.Link, task.GroupIDs)

		// Mark task as in progress immediately to prevent duplicate processing
		log.Printf("Marking task %d as in progress", task.ID)
		if err := s.taskRepo.MarkTaskInProgress(task.ID); err != nil {
			log.Printf("Failed to mark task %d as in progress: %v", task.ID, err)
			continue
		}
		log.Printf("Successfully marked task %d as in progress", task.ID)

		// Try to queue the task (non-blocking)
		log.Printf("Attempting to queue task %d", task.ID)
		select {
		case s.taskQueue <- VideoTask{ID: task.ID, Link: task.Link, GroupIDs: task.GroupIDs}:
			// Task queued successfully
			log.Printf("Queued task %d for processing in %d groups", task.ID, len(task.GroupIDs))
		default:
			// Queue is full, task will be picked up in next cycle
			log.Printf("Task queue is full, task %d will be processed in next cycle", task.ID)
			break
		}
	}

	if taskCount == 0 {
		log.Printf("scheduleAvailableTasks completed - no tasks found")
	} else {
		log.Printf("scheduleAvailableTasks completed - processed %d tasks", taskCount)
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
			log.Printf("Worker processing task %d for groups %v", task.ID, task.GroupIDs)
			s.processTask(task.ID, task.Link, task.GroupIDs)
		}
	}
}

func (s *VideoService) processTask(taskID int, link string, groupIDs []int64) {
	log.Printf("Starting to process task %d with link: %s for groups: %v", taskID, link, groupIDs)

	_, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Validate URL first
	isValid, platformName, err := s.downloadRepo.ValidateURL(link)
	if err != nil || !isValid {
		log.Printf("URL validation failed for task %d: %v", taskID, err)
		s.handleTaskFailure(taskID, groupIDs, fmt.Sprintf("Invalid URL: %v", err))
		return
	}

	log.Printf("Processing %s video: %s", platformName, link)

	// Download video to a shared directory (use first group ID for directory)
	outputDir := fmt.Sprintf("%s/shared", core.VideoOutputDirectory)
	log.Printf("Starting download for task %d to directory: %s", taskID, outputDir)

	result, err := s.downloadRepo.DownloadVideo(link, outputDir)
	if err != nil || !result.Success {
		log.Printf("Download failed for task %d: %v", taskID, err)
		if result != nil {
			log.Printf("Download result error: %v", result.Error)
		}
		s.handleTaskFailure(taskID, groupIDs, fmt.Sprintf("Download failed: %v", result.Error))
		return
	}

	log.Printf("Download successful for task %d, file: %s", taskID, result.FilePath)

	// Upload to all groups
	uploadCount := 0
	for _, groupID := range groupIDs {
		log.Printf("Uploading to group %d", groupID)
		err = s.uploadRepo.UploadVideo(result.FilePath, groupID)
		if err != nil {
			log.Printf("Failed to upload to group %d: %v", groupID, err)
			// Continue uploading to other groups
		} else {
			uploadCount++
			log.Printf("Successfully uploaded to group %d", groupID)
		}
	}

	// Success - clean up and notify all groups
	log.Printf("Cleaning up file: %s", result.FilePath)
	os.Remove(result.FilePath)

	log.Printf("Calling success handler for task %d with %d successful uploads", taskID, uploadCount)
	s.handleTaskSuccess(taskID, groupIDs, result.FileName)
}

func (s *VideoService) handleTaskSuccess(taskID int, groupIDs []int64, fileName string) {
	log.Printf("handleTaskSuccess called for task %d, groups %v, file %s", taskID, groupIDs, fileName)

	// Delete completed task
	if err := s.taskRepo.DeleteTask(taskID); err != nil {
		log.Printf("Failed to delete completed task %d: %v", taskID, err)
	} else {
		log.Printf("Successfully deleted completed task %d", taskID)
	}

	// Emit success events for all groups
	for _, groupID := range groupIDs {
		log.Printf("Emitting success event for group %d with fileName=%s", groupID, fileName)
		select {
		case s.eventChannel <- entity.VideoProcessSuccess{GroupID: groupID, FileName: fileName}:
			log.Printf("Successfully emitted success event for group %d", groupID)
		default:
			log.Printf("WARNING: Event channel is full, dropping success event for group %d", groupID)
		}
	}

	log.Printf("Successfully processed video for groups %v: %s", groupIDs, fileName)
}

func (s *VideoService) handleTaskFailure(taskID int, groupIDs []int64, errorMessage string) {
	log.Printf("handleTaskFailure called for task %d, groups %v, error: %s", taskID, groupIDs, errorMessage)

	// Delete failed task
	if err := s.taskRepo.DeleteTask(taskID); err != nil {
		log.Printf("Failed to delete failed task %d: %v", taskID, err)
	} else {
		log.Printf("Successfully deleted failed task %d", taskID)
	}

	// Emit failure events for all groups
	for _, groupID := range groupIDs {
		log.Printf("Emitting failure event for group %d with error=%s", groupID, errorMessage)
		select {
		case s.eventChannel <- entity.VideoProcessFailure{GroupID: groupID, ErrorMessage: errorMessage}:
			log.Printf("Successfully emitted failure event for group %d", groupID)
		default:
			log.Printf("WARNING: Event channel is full, dropping failure event for group %d", groupID)
		}
	}

	log.Printf("Failed to process video for groups %v: %s", groupIDs, errorMessage)
}
