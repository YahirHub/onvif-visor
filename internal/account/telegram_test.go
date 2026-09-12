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
