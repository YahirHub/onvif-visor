package telegram

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGetMeValidatesBot(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/bottest-token/getMe" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"result":{"id":42,"is_bot":true,"first_name":"Visor","username":"visor_test_bot"}}`))
	}))
	defer server.Close()

	client := NewForTests(server.URL, server.Client())
	bot, err := client.GetMe(context.Background(), "test-token")
	if err != nil {
		t.Fatal(err)
	}
	if bot.ID != 42 || bot.Username != "visor_test_bot" {
		t.Fatalf("unexpected bot: %#v", bot)
	}
}

func TestGetMeRejectsTelegramError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":false,"description":"Unauthorized"}`))
	}))
	defer server.Close()

	client := NewForTests(server.URL, server.Client())
	if _, err := client.GetMe(context.Background(), "bad-token"); err == nil {
		t.Fatal("expected GetMe error")
	}
}

func TestTransportErrorDoesNotExposeToken(t *testing.T) {
	const secret = "123456:SUPER_SECRET_TOKEN"
	client := NewForTests("http://127.0.0.1:1", &http.Client{})
	_, err := client.GetMe(context.Background(), secret)
	if err == nil {
		t.Fatal("expected transport error")
	}
	if strings.Contains(err.Error(), secret) {
		t.Fatalf("transport error exposed token: %v", err)
	}
}
