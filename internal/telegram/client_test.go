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

func TestSendPhotoWithCallbackButtonIncludesInlineKeyboard(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/bottest-token/sendPhoto" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Fatal(err)
		}
		markup := r.FormValue("reply_markup")
		if !strings.Contains(markup, "Ver video") || !strings.Contains(markup, "pv:abc123") {
			t.Fatalf("unexpected reply_markup: %s", markup)
		}
		if r.FormValue("caption") != "Persona detectada" {
			t.Fatalf("unexpected caption %q", r.FormValue("caption"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"result":{"message_id":1}}`))
	}))
	defer server.Close()

	client := NewForTests(server.URL, server.Client())
	if err := client.SendPhotoWithCallbackButton(
		context.Background(), "test-token", 123, []byte("jpeg"), "Persona detectada", "Ver video", "pv:abc123",
	); err != nil {
		t.Fatal(err)
	}
}

func TestGetUpdatesRequestsCallbackQueries(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(r.FormValue("allowed_updates"), "callback_query") {
			t.Fatalf("allowed_updates=%q", r.FormValue("allowed_updates"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"result":[{"update_id":9,"callback_query":{"id":"cb1","from":{"id":77},"message":{"message_id":1,"chat":{"id":123},"text":""},"data":"pv:abc"}}]}`))
	}))
	defer server.Close()

	client := NewForTests(server.URL, server.Client())
	updates, err := client.GetUpdates(context.Background(), "test-token", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(updates) != 1 || updates[0].CallbackQuery == nil || updates[0].CallbackQuery.Data != "pv:abc" {
		t.Fatalf("unexpected updates: %#v", updates)
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
