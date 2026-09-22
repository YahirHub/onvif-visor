package account

import (
	"context"
	"testing"
)

func TestTelegramCodeIsSingleUseAndConfigPersists(t *testing.T) {
	ctx := context.Background()
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	user, err := s.EnsureBootstrapUser(ctx, "demo", "demo")
	if err != nil {
		t.Fatal(err)
	}
	cfg := TelegramConfig{
		TokenCipher: "encrypted-token", BotID: 42, BotUsername: "visor_test_bot",
		NotificationsEnabled: true, MediaMode: "photo", VideoSeconds: 5,
	}
	if err := s.SaveTelegramConfig(ctx, cfg); err != nil {
		t.Fatal(err)
	}
	loaded, ok, err := s.GetTelegramConfig(ctx)
	if err != nil || !ok {
		t.Fatalf("GetTelegramConfig() = %#v, %v, %v", loaded, ok, err)
	}
	if loaded.BotID != 42 || loaded.BotUsername != "visor_test_bot" || loaded.TokenCipher != "encrypted-token" {
		t.Fatalf("unexpected telegram config: %#v", loaded)
	}
	code, _, err := s.CreateTelegramCode(ctx, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(code) != 4 {
		t.Fatalf("code length = %d, want 4", len(code))
	}
	userID, consumed, err := s.ConsumeTelegramCode(ctx, code)
	if err != nil || !consumed || userID != user.ID {
		t.Fatalf("consume = user %d ok=%v err=%v", userID, consumed, err)
	}
	if _, consumed, err := s.ConsumeTelegramCode(ctx, code); err != nil || consumed {
		t.Fatalf("code was reusable: ok=%v err=%v", consumed, err)
	}
	if err := s.LinkTelegramChat(ctx, user.ID, 12345, "Mi chat"); err != nil {
		t.Fatal(err)
	}
	chats, err := s.ListTelegramChats(ctx)
	if err != nil || len(chats) != 1 || chats[0].ChatID != 12345 {
		t.Fatalf("unexpected chats: %#v err=%v", chats, err)
	}
	if err := s.DeleteTelegramConfig(ctx); err != nil {
		t.Fatal(err)
	}
	_, ok, err = s.GetTelegramConfig(ctx)
	if err != nil || ok {
		t.Fatalf("telegram config still exists: ok=%v err=%v", ok, err)
	}
}

func TestTelegramPhotoVideoModeAndMediaActionArePersistent(t *testing.T) {
	ctx := context.Background()
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	cfg := TelegramConfig{
		TokenCipher: "cipher", BotID: 7, BotUsername: "bot",
		NotificationsEnabled: true, MediaMode: "photo_video", VideoSeconds: 12,
	}
	if err := s.SaveTelegramConfig(ctx, cfg); err != nil {
		t.Fatalf("SaveTelegramConfig(photo_video) error = %v", err)
	}
	loaded, ok, err := s.GetTelegramConfig(ctx)
	if err != nil || !ok || loaded.MediaMode != "photo_video" || loaded.VideoSeconds != 12 {
		t.Fatalf("unexpected loaded config: %#v ok=%v err=%v", loaded, ok, err)
	}
	user, err := s.EnsureBootstrapUser(ctx, "demo", "demo")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.LinkTelegramChat(ctx, user.ID, 12345, "Mi chat"); err != nil {
		t.Fatal(err)
	}

	action, err := s.CreateTelegramMediaAction(ctx, 12345, "cam-1", "clip.mkv", 8.25)
	if err != nil {
		t.Fatal(err)
	}
	if len(action.ID) != 16 {
		t.Fatalf("media action id length = %d, want 16", len(action.ID))
	}
	got, found, err := s.GetTelegramMediaAction(ctx, action.ID, 12345)
	if err != nil || !found {
		t.Fatalf("GetTelegramMediaAction() = %#v found=%v err=%v", got, found, err)
	}
	if got.CameraID != "cam-1" || got.RecordingName != "clip.mkv" || got.OffsetSecond != 8.25 {
		t.Fatalf("unexpected media action: %#v", got)
	}
	if _, found, err := s.GetTelegramMediaAction(ctx, action.ID, 999); err != nil || found {
		t.Fatalf("media action leaked to another chat: found=%v err=%v", found, err)
	}

	if err := s.DeleteTelegramConfig(ctx); err != nil {
		t.Fatal(err)
	}
	if _, found, err := s.GetTelegramMediaAction(ctx, action.ID, 12345); err != nil || found {
		t.Fatalf("media action survived integration deletion: found=%v err=%v", found, err)
	}
}

func TestTelegramVideoDurationSupportsThirtySeconds(t *testing.T) {
	ctx := context.Background()
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	cfg := TelegramConfig{TokenCipher: "cipher", BotID: 1, BotUsername: "bot", NotificationsEnabled: true, MediaMode: "video", VideoSeconds: 30}
	if err := s.SaveTelegramConfig(ctx, cfg); err != nil {
		t.Fatalf("SaveTelegramConfig(30s) error = %v", err)
	}
	if err := s.UpdateTelegramSettings(ctx, true, "video", 60); err != nil {
		t.Fatalf("UpdateTelegramSettings(60s) error = %v", err)
	}
	if err := s.UpdateTelegramSettings(ctx, true, "video", 61); err == nil {
		t.Fatal("expected 61s duration to fail")
	}
}
