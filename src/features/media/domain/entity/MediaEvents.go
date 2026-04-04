package entity

// MediaEvents is the channel for getting media processing events
type MediaEvents <-chan MediaEvent

// MediaEvent is the sealed interface for all media processing events
type MediaEvent interface {
	isMediaEvent()
}

// MediaUploadStarted event for when media upload to Telegram starts
type MediaUploadStarted struct {
	GroupID   int64
	MessageID int
}

func (MediaUploadStarted) isMediaEvent() {}

// MediaProcessSuccess event for successful media processing
type MediaProcessSuccess struct {
	GroupID   int64
	MessageID int
}

func (MediaProcessSuccess) isMediaEvent() {}

// MediaProcessFailure event for failed media processing
type MediaProcessFailure struct {
	GroupID      int64
	MessageID    int
	ErrorMessage string
}

func (MediaProcessFailure) isMediaEvent() {}
