package entity

// MediaFile represents a single media file
type MediaFile struct {
	FilePath  string
	FileName  string
	FileSize  int64
	MediaType MediaType
}

// MediaProcessResult represents the result of a media processing operation
type MediaProcessResult struct {
	Success bool
	Files   []MediaFile
	Error   error
	GroupID int64
}
