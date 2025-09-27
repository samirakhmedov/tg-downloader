package controller

import (
	"log"
	"tg-downloader/src/features/bot/domain/entity"
	"tg-downloader/src/features/bot/domain/service"
	videoEntity "tg-downloader/src/features/video/domain/entity"
	videoService "tg-downloader/src/features/video/domain/service"
)

type IBotController interface {
	Initialize()
	Dispose()
}

type BotController struct {
	service      service.IBotService
	videoService videoService.IVideoService
}

func NewBotController(service service.IBotService, videoService videoService.IVideoService) *BotController {
	controller := &BotController{
		service:      service,
		videoService: videoService,
	}

	return controller
}

func (c *BotController) Initialize() {
	go c.videoService.StartWorkers()
	go c.processEvents()
	go c.processVideoEvents()
}

func (c *BotController) Dispose() {
	c.videoService.StopWorkers()
}

func (c *BotController) processEvents() {
	events := c.service.GetBotEvents()

	for event := range events {
		go c.handleEvent(event)
	}
}

func (c *BotController) handleEvent(event entity.BotEvent) {
	c.updateCommands(event)
	c.handleBusinessLogic(event)
}

func (c *BotController) updateCommands(event entity.BotEvent) {
	switch e := event.(type) {
	case entity.DirectGetBotCommands, entity.GetServerLoad, entity.GetAllGroups, entity.DeleteGroup, entity.ErrorDirect, entity.GetResource:
		c.updateDirectCommands(e)
	case entity.GroupGetBotCommands, entity.ActivateGroup, entity.DeactivateGroup, entity.ErrorGroup:
		c.updateGroupCommands(e)
	case entity.IgnoreCommand:
		if e.GroupID == 0 {
			c.updateDirectCommands(e)
		} else {
			c.updateGroupCommands(e)
		}
	}
}

func (c *BotController) updateDirectCommands(event entity.BotEvent) {
	switch e := event.(type) {
	case entity.DirectGetBotCommands:
		c.service.UpdateCommandsForUser(e.UserID, e.UserName)
	case entity.GetServerLoad:
		c.service.UpdateCommandsForUser(e.UserID, e.UserName)
	case entity.GetAllGroups:
		c.service.UpdateCommandsForUser(e.UserID, e.UserName)
	case entity.DeleteGroup:
		c.service.UpdateCommandsForUser(e.UserID, e.UserName)
	case entity.IgnoreCommand:
		c.service.UpdateCommandsForUser(e.UserID, e.UserName)
	}
}

func (c *BotController) updateGroupCommands(event entity.BotEvent) {
	switch e := event.(type) {
	case entity.GroupGetBotCommands:
		c.service.UpdateCommandsForGroupUser(e.GroupID, e.UserID, e.UserName)
	case entity.ActivateGroup:
		c.service.UpdateCommandsForGroupUser(e.GroupID, e.UserID, e.UserName)
	case entity.DeactivateGroup:
		c.service.UpdateCommandsForGroupUser(e.GroupID, e.UserID, e.UserName)
	case entity.IgnoreCommand:
		c.service.UpdateCommandsForGroupUser(e.GroupID, e.UserID, e.UserName)
	}
}

func (c *BotController) handleBusinessLogic(event entity.BotEvent) {
	switch e := event.(type) {
	case entity.ActivateGroup:
		c.service.ActivateGroup(e.GroupID, e.UserID, e.UserName)
	case entity.DeactivateGroup:
		c.service.DeactivateGroup(e.GroupID, e.UserID, e.UserName)
	case entity.GetServerLoad:
		c.service.GetServerLoad(e.UserID, e.UserName)
	case entity.GetAllGroups:
		c.service.GetAllGroups(e.UserID, e.UserName)
	case entity.DeleteGroup:
		c.service.DeleteGroup(e.GroupID, e.UserID, e.UserName)
	case entity.StartBot:
		c.service.GetDirectCommands(e.UserID, e.UserName)
	case entity.GetResource:
		canProcess, err := c.service.LoadResource(e.GroupID, e.Link)
		if err != nil {
			// Error already handled by service (message sent to user)
			return
		}
		if canProcess {
			// Start video processing
			c.videoService.ProcessVideo(e.Link, e.GroupID)
		}
	case entity.DirectGetBotCommands:
		c.service.GetDirectCommands(e.UserID, e.UserName)
	case entity.GroupGetBotCommands:
		c.service.GetGroupCommands(e.GroupID, e.UserID, e.UserName)
	case entity.ErrorDirect:
		c.service.HandleDirectError(e.UserID, e.UserName, e.Message)
	case entity.ErrorGroup:
		c.service.HandleGroupError(e.GroupID, e.Message)
	case entity.IgnoreCommand:
		// Do nothing for ignored commands
	default:
		// Unknown event type - log it but don't crash
		_ = e
	}
}

func (c *BotController) processVideoEvents() {
	videoEvents := c.videoService.GetVideoEvents()

	for event := range videoEvents {
		go c.handleVideoEvent(event)
	}
}

func (c *BotController) handleVideoEvent(event videoEntity.VideoEvent) {
	switch e := event.(type) {
	case videoEntity.VideoProcessSuccess:
		log.Printf("Received video success event for group %d with fileName=%s", e.GroupID, e.FileName)
		err := c.service.HandleVideoProcessSuccess(e.GroupID, e.FileName)
		if err != nil {
			log.Printf("HandleVideoProcessSuccess failed: %v", err)
		} else {
			log.Printf("HandleVideoProcessSuccess completed successfully")
		}
	case videoEntity.VideoProcessFailure:
		log.Printf("Received video failure event for group %d with error=%s", e.GroupID, e.ErrorMessage)
		err := c.service.HandleVideoProcessFailure(e.GroupID, e.ErrorMessage)
		if err != nil {
			log.Printf("HandleVideoProcessFailure failed: %v", err)
		} else {
			log.Printf("HandleVideoProcessFailure completed successfully")
		}
	default:
		log.Printf("Unknown video event type: %T", e)
	}
}
