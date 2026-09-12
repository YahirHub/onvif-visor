package onvifsvc

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
)

func CredentialedRTSP(raw, username, password string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	if u.Scheme != "rtsp" && u.Scheme != "rtsps" {
		return "", errors.New("stream URI must use rtsp or rtsps")
	}
	if u.Host == "" {
		return "", errors.New("stream URI has no host")
	}
	if username != "" {
		u.User = url.UserPassword(username, password)
	}
	return u.String(), nil
}

func normalizeHTTPURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", errors.New("camera endpoint is required")
	}
	if !strings.Contains(raw, "://") {
		raw = "http://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("parse endpoint: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", errors.New("camera endpoint must use http or https")
	}
	if u.Host == "" {
		return "", errors.New("camera endpoint has no host")
	}
	u.User = nil
	if u.Path == "" || u.Path == "/" {
		u.Path = "/onvif/device_service"
	}
	return u.String(), nil
}

func normalizeRTSPURL(raw string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "", err
	}
	if u.Scheme != "rtsp" && u.Scheme != "rtsps" {
		return "", errors.New("unsupported stream scheme")
	}
	if u.Host == "" {
		return "", errors.New("stream URI has no host")
	}
	u.User = nil
	return u.String(), nil
}

func hostLabel(endpoint string) string {
	u, err := url.Parse(endpoint)
	if err != nil {
		return endpoint
	}
	return u.Hostname()
}
