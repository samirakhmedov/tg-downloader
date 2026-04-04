package repository_test

import (
	"context"
	"testing"

	mediaRepo "tg-downloader/src/features/media/data/repository"
	mediaEntity "tg-downloader/src/features/media/domain/entity"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// mockSender is a test double for BotAPISender.
type mockSender struct {
	sendCalls      int
	sendGroupCalls int
	lastSentConfig tgbotapi.Chattable
	lastMediaGroup []interface{}
	sendErr        error
	sendGroupErr   error
}

func (m *mockSender) Send(c tgbotapi.Chattable) (tgbotapi.Message, error) {
	m.sendCalls++
	m.lastSentConfig = c
	return tgbotapi.Message{}, m.sendErr
}

func (m *mockSender) SendMediaGroup(config tgbotapi.MediaGroupConfig) ([]tgbotapi.Message, error) {
	m.sendGroupCalls++
	m.lastMediaGroup = config.Media
	return []tgbotapi.Message{}, m.sendGroupErr
}

// newRepo is a helper to create a MediaUploadRepository with a mock sender.
func newRepo(sender *mockSender) *mediaRepo.MediaUploadRepository {
	return mediaRepo.NewMediaUploadRepositoryWithSender(sender)
}

const testGroupID int64 = 12345

// TestUploadPhoto verifies that UploadPhoto calls Send once with a PhotoConfig.
func TestUploadPhoto(t *testing.T) {
	mock := &mockSender{}
	repo := newRepo(mock)

	err := repo.UploadPhoto(context.Background(), "/tmp/test.jpg", testGroupID)
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
	if mock.sendCalls != 1 {
		t.Errorf("expected Send called 1 time, got %d", mock.sendCalls)
	}
	if mock.sendGroupCalls != 0 {
		t.Errorf("expected SendMediaGroup not called, got %d", mock.sendGroupCalls)
	}
}

// TestUploadPhotoError verifies that UploadPhoto propagates errors from Send.
func TestUploadPhotoError(t *testing.T) {
	mock := &mockSender{sendErr: errSend}
	repo := newRepo(mock)

	err := repo.UploadPhoto(context.Background(), "/tmp/test.jpg", testGroupID)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

var errSend = &testError{"send error"}
var errSendGroup = &testError{"send group error"}

type testError struct{ msg string }

func (e *testError) Error() string { return e.msg }

// TestUploadAlbumChunking verifies that 15 photo items result in exactly 2 SendMediaGroup calls.
func TestUploadAlbumChunking(t *testing.T) {
	mock := &mockSender{}
	repo := newRepo(mock)

	items := make([]mediaEntity.MediaItem, 15)
	for i := range items {
		items[i] = mediaEntity.MediaItem{FilePath: "/tmp/photo.jpg", Type: mediaEntity.MediaTypePhoto, Order: i}
	}

	err := repo.UploadAlbum(context.Background(), items, testGroupID)
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
	if mock.sendGroupCalls != 2 {
		t.Errorf("expected SendMediaGroup called 2 times, got %d", mock.sendGroupCalls)
	}
	if mock.sendCalls != 0 {
		t.Errorf("expected Send not called, got %d", mock.sendCalls)
	}
}

// TestUploadAlbumMixedMedia verifies that mixed photo+video items are correctly typed.
func TestUploadAlbumMixedMedia(t *testing.T) {
	mock := &mockSender{}
	repo := newRepo(mock)

	items := []mediaEntity.MediaItem{
		{FilePath: "/tmp/photo1.jpg", Type: mediaEntity.MediaTypePhoto, Order: 0},
		{FilePath: "/tmp/photo2.jpg", Type: mediaEntity.MediaTypePhoto, Order: 1},
		{FilePath: "/tmp/video.mp4", Type: mediaEntity.MediaTypeVideo, Order: 2},
	}

	err := repo.UploadAlbum(context.Background(), items, testGroupID)
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
	if mock.sendGroupCalls != 1 {
		t.Errorf("expected SendMediaGroup called 1 time, got %d", mock.sendGroupCalls)
	}
	if len(mock.lastMediaGroup) != 3 {
		t.Fatalf("expected 3 items in media group, got %d", len(mock.lastMediaGroup))
	}

	// First two should be InputMediaPhoto
	for i := 0; i < 2; i++ {
		if _, ok := mock.lastMediaGroup[i].(tgbotapi.InputMediaPhoto); !ok {
			t.Errorf("item %d: expected InputMediaPhoto, got %T", i, mock.lastMediaGroup[i])
		}
	}

	// Third should be InputMediaVideo with SupportsStreaming=true
	video, ok := mock.lastMediaGroup[2].(tgbotapi.InputMediaVideo)
	if !ok {
		t.Fatalf("item 2: expected InputMediaVideo, got %T", mock.lastMediaGroup[2])
	}
	if !video.SupportsStreaming {
		t.Error("expected InputMediaVideo.SupportsStreaming=true")
	}
}

// TestUploadAlbumSingleItem verifies that a single photo item routes to Send (not SendMediaGroup).
func TestUploadAlbumSingleItem(t *testing.T) {
	mock := &mockSender{}
	repo := newRepo(mock)

	items := []mediaEntity.MediaItem{
		{FilePath: "/tmp/photo.jpg", Type: mediaEntity.MediaTypePhoto, Order: 0},
	}

	err := repo.UploadAlbum(context.Background(), items, testGroupID)
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
	if mock.sendCalls != 1 {
		t.Errorf("expected Send called 1 time, got %d", mock.sendCalls)
	}
	if mock.sendGroupCalls != 0 {
		t.Errorf("expected SendMediaGroup not called, got %d", mock.sendGroupCalls)
	}
}

// TestUploadAlbumSingleVideo verifies that a single video item routes to Send with SupportsStreaming=true.
func TestUploadAlbumSingleVideo(t *testing.T) {
	mock := &mockSender{}
	repo := newRepo(mock)

	items := []mediaEntity.MediaItem{
		{FilePath: "/tmp/video.mp4", Type: mediaEntity.MediaTypeVideo, Order: 0},
	}

	err := repo.UploadAlbum(context.Background(), items, testGroupID)
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
	if mock.sendCalls != 1 {
		t.Errorf("expected Send called 1 time, got %d", mock.sendCalls)
	}
	if mock.sendGroupCalls != 0 {
		t.Errorf("expected SendMediaGroup not called, got %d", mock.sendGroupCalls)
	}
}

// TestUploadVideo verifies that UploadVideo calls Send once with a VideoConfig.
func TestUploadVideo(t *testing.T) {
	mock := &mockSender{}
	repo := newRepo(mock)

	err := repo.UploadVideo(context.Background(), "/tmp/test.mp4", testGroupID)
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
	if mock.sendCalls != 1 {
		t.Errorf("expected Send called 1 time, got %d", mock.sendCalls)
	}
	if mock.sendGroupCalls != 0 {
		t.Errorf("expected SendMediaGroup not called, got %d", mock.sendGroupCalls)
	}
}

// TestUploadVideoError verifies that UploadVideo propagates errors from Send.
func TestUploadVideoError(t *testing.T) {
	mock := &mockSender{sendErr: errSend}
	repo := newRepo(mock)

	err := repo.UploadVideo(context.Background(), "/tmp/test.mp4", testGroupID)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// TestUploadAlbumEmpty verifies that 0 items returns nil immediately without any API calls.
func TestUploadAlbumEmpty(t *testing.T) {
	mock := &mockSender{}
	repo := newRepo(mock)

	err := repo.UploadAlbum(context.Background(), []mediaEntity.MediaItem{}, testGroupID)
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
	if mock.sendCalls != 0 {
		t.Errorf("expected Send not called, got %d", mock.sendCalls)
	}
	if mock.sendGroupCalls != 0 {
		t.Errorf("expected SendMediaGroup not called, got %d", mock.sendGroupCalls)
	}
}
