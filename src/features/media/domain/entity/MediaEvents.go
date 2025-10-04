package entity

// MediaEvents is the channel for getting media processing events
type MediaEvents <-chan MediaEvent

// MediaEvent is the sealed interface for all media processing events
type MediaEvent interface {
	isMediaEvent()
}

// MediaProcessSuccess event for successful media processing
type MediaProcessSuccess struct {
	GroupID   int64
	FileNames []string // List of filenames for all processed media
}

func (MediaProcessSuccess) isMediaEvent() {}

// MediaProcessFailure event for failed media processing
type MediaProcessFailure struct {
	GroupID      int64
	ErrorMessage string
}

func (MediaProcessFailure) isMediaEvent() {}
