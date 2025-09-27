package repository

type IUploadRepository interface {
	UploadVideo(filePath string, groupID int64) error
}
