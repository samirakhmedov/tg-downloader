package entity

// VideoProcessResult represents the result of a video processing operation
type VideoProcessResult struct {
	Success   bool
	FilePath  string
	Error     error
	GroupID   int64
	FileName  string
	FileSize  int64
}