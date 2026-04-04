package entity

type MediaType string

const (
	MediaTypePhoto MediaType = "photo"
	MediaTypeVideo MediaType = "video"
)

type MediaItem struct {
	FilePath string
	Type     MediaType
	Order    int
}
