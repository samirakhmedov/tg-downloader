package repository

import "tg-downloader/src/features/system/domain/entity"

type ISystemRepository interface {
	GetSystemInfo() (*entity.SystemInfo, error)
}