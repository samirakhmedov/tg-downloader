package entity

// DownloadResult represents the result of a media download operation
type DownloadResult struct {
	Success  bool
	FilePath string
	Error    error
	GroupID  int64
	FileName string
	FileSize int64
}
