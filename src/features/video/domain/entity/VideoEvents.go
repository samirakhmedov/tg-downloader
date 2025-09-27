package entity

// VideoEvents is the channel for getting video processing events
type VideoEvents <-chan VideoEvent

// VideoEvent is the sealed interface for all video processing events
type VideoEvent interface {
	isVideoEvent()
}

// VideoProcessSuccess event for successful video processing
type VideoProcessSuccess struct {
	GroupID  int64
	FileName string
}

func (VideoProcessSuccess) isVideoEvent() {}

// VideoProcessFailure event for failed video processing
type VideoProcessFailure struct {
	GroupID      int64
	ErrorMessage string
}

func (VideoProcessFailure) isVideoEvent() {}