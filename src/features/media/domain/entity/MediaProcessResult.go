package entity

type MediaProcessResult struct {
	Items   []MediaItem
	Success bool
	Error   error
}
