package controller

import (
	"fmt"
	"strings"
	"tg-downloader/src/core/logger"
	"tg-downloader/src/features/bot/domain/entity"
	"tg-downloader/src/features/bot/domain/service"
	mediaEntity "tg-downloader/src/features/media/domain/entity"
	mediaService "tg-downloader/src/features/media/domain/service"
)

type IBotController interface {
	Initialize()
	Dispose()
}

type BotController struct {
	service      service.IBotService
	mediaService mediaService.IMediaService
	logger       *logger.Logger
}

// NewBotController creates a new BotController with all required dependencies.
// The logger parameter allows dynamic control of logging behavior throughout event processing.
func NewBotController(service service.IBotService, mediaService mediaService.IMediaService, logger *logger.Logger) *BotController {
	controller := &BotController{
		service:      service,
		mediaService: mediaService,
		logger:       logger,
	}

	return controller
}

func (c *BotController) Initialize() {
	go c.mediaService.StartWorkers()
	go c.processEvents()
	go c.processMediaEvents()
}

func (c *BotController) Dispose() {
	c.mediaService.StopWorkers()
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
			// Start media processing
			c.mediaService.ProcessMedia(e.Link, e.GroupID)
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

func (c *BotController) processMediaEvents() {
	mediaEvents := c.mediaService.GetMediaEvents()

	for event := range mediaEvents {
		go c.handleMediaEvent(event)
	}
}

func (c *BotController) handleMediaEvent(event mediaEntity.MediaEvent) {
	switch e := event.(type) {
	case mediaEntity.MediaProcessSuccess:
		fileNamesList := strings.Join(e.FileNames, ", ")
		c.logger.Debug(fmt.Sprintf("Received media success event for group %d with fileNames=%s", e.GroupID, fileNamesList))

		// For backward compatibility, use the first filename if available
		fileName := ""
		if len(e.FileNames) > 0 {
			fileName = e.FileNames[0]
		}

		err := c.service.HandleVideoProcessSuccess(e.GroupID, fileName)
		if err != nil {
			c.logger.Error(fmt.Sprintf("HandleVideoProcessSuccess failed: %v", err))
		} else {
			c.logger.Debug("HandleVideoProcessSuccess completed successfully")
		}
	case mediaEntity.MediaProcessFailure:
		c.logger.Debug(fmt.Sprintf("Received media failure event for group %d with error=%s", e.GroupID, e.ErrorMessage))
		err := c.service.HandleVideoProcessFailure(e.GroupID, e.ErrorMessage)
		if err != nil {
			c.logger.Error(fmt.Sprintf("HandleVideoProcessFailure failed: %v", err))
		} else {
			c.logger.Debug("HandleVideoProcessFailure completed successfully")
		}
	default:
		c.logger.Warn(fmt.Sprintf("Unknown media event type: %T", e))
	}
}
