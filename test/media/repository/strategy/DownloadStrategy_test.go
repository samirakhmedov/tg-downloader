package strategy_test

import (
	"testing"

	"github.com/lrstanley/go-ytdlp"
	"tg-downloader/src/features/media/data/repository/strategy"
)

func TestDownloadStrategyName(t *testing.T) {
	s := strategy.NewDownloadStrategy("720p mp4", "22/bestvideo[height<=720]", []string{"youtube:player_client=android_sdkless"})
	if s.Name() != "720p mp4" {
		t.Errorf("expected Name() = %q, got %q", "720p mp4", s.Name())
	}
}

func TestDownloadStrategyAppliesExtractorArgs(t *testing.T) {
	s := strategy.NewDownloadStrategy("best", "b/best", []string{"youtube:player_client=android_sdkless"})
	cmd := ytdlp.New()
	result := s.Apply(cmd)
	if result == nil {
		t.Fatal("Apply() returned nil command")
	}
}

func TestDownloadStrategyEmptyExtractorArgs(t *testing.T) {
	s := strategy.NewDownloadStrategy("best", "b/best", nil)
	cmd := ytdlp.New()
	result := s.Apply(cmd)
	if result == nil {
		t.Fatal("Apply() with nil extractorArgs returned nil command")
	}
}

func TestDownloadStrategyMultipleExtractorArgs(t *testing.T) {
	s := strategy.NewDownloadStrategy("best", "b/best", []string{
		"youtube:player_client=android_sdkless",
		"tiktok:api_hostname=api.tiktokv.com",
	})
	cmd := ytdlp.New()
	result := s.Apply(cmd)
	if result == nil {
		t.Fatal("Apply() with multiple extractorArgs returned nil command")
	}
}
