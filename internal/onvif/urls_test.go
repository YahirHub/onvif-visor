package onvifsvc

import (
	"net/url"
	"strings"
	"testing"
)

func TestNormalizeHTTPURLAddsDefaultDeviceService(t *testing.T) {
	got, err := normalizeHTTPURL("192.0.2.20:8080")
	if err != nil {
		t.Fatal(err)
	}
	if got != "http://192.0.2.20:8080/onvif/device_service" {
		t.Fatalf("normalizeHTTPURL() = %q", got)
	}
}

func TestNormalizeRTSPURLRemovesCredentials(t *testing.T) {
	got, err := normalizeRTSPURL("rtsp://admin:secret@192.0.2.20/live")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(got, "admin") || strings.Contains(got, "secret") {
		t.Fatalf("normalizeRTSPURL() kept credentials: %q", got)
	}
}

func TestCredentialedRTSPEscapesCredentials(t *testing.T) {
	got, err := CredentialedRTSP("rtsp://192.0.2.20/live", "user@example", "p@ss word")
	if err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(got)
	if err != nil {
		t.Fatal(err)
	}
	if u.User == nil || u.User.Username() != "user@example" {
		t.Fatalf("unexpected user info: %#v", u.User)
	}
	password, ok := u.User.Password()
	if !ok || password != "p@ss word" {
		t.Fatalf("unexpected password: %q, %v", password, ok)
	}
}
