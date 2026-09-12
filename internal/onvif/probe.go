package onvifsvc

import (
	"context"
	"errors"
	"fmt"
	"time"

	onvif "github.com/0x524a/onvif-go"
)

const operationTimeout = 12 * time.Second

func Probe(ctx context.Context, creds Credentials) (ProbeResult, error) {
	endpoint, err := normalizeHTTPURL(creds.Endpoint)
	if err != nil {
		return ProbeResult{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, operationTimeout)
	defer cancel()

	client, err := newClient(endpoint, creds.Username, creds.Password)
	if err != nil {
		return ProbeResult{}, err
	}
	if err := client.Initialize(ctx); err != nil {
		return ProbeResult{}, fmt.Errorf("initialize ONVIF services: %w", err)
	}
	info, err := client.GetDeviceInformation(ctx)
	if err != nil {
		return ProbeResult{}, fmt.Errorf("read device information: %w", err)
	}
	profiles, err := client.GetProfiles(ctx)
	if err != nil {
		return ProbeResult{}, fmt.Errorf("read media profiles: %w", err)
	}
	if len(profiles) == 0 {
		return ProbeResult{}, errors.New("camera has no ONVIF media profiles")
	}
	profile := chooseProfile(profiles)
	mediaURI, err := client.GetStreamURI(ctx, profile.Token)
	if err != nil {
		return ProbeResult{}, fmt.Errorf("read RTSP stream URI: %w", err)
	}
	rtspURI, err := normalizeRTSPURL(mediaURI.URI)
	if err != nil {
		return ProbeResult{}, fmt.Errorf("invalid RTSP stream URI: %w", err)
	}

	eventsSupported := false
	caps, capErr := client.GetCapabilities(ctx)
	if capErr == nil && caps.Events != nil {
		eventsSupported = caps.Events.WSPullPointSupport || caps.Events.XAddr != ""
	}
	return ProbeResult{
		Endpoint:        endpoint,
		Manufacturer:    info.Manufacturer,
		Model:           info.Model,
		FirmwareVersion: info.FirmwareVersion,
		ProfileToken:    profile.Token,
		ProfileName:     profile.Name,
		RTSPURI:         rtspURI,
		EventsSupported: eventsSupported,
	}, nil
}

func newClient(endpoint, username, password string) (*onvif.Client, error) {
	return onvif.NewClient(endpoint,
		onvif.WithCredentials(username, password),
		onvif.WithTimeout(operationTimeout),
	)
}

func chooseProfile(profiles []*onvif.Profile) *onvif.Profile {
	best := profiles[0]
	bestPixels := -1
	for _, profile := range profiles {
		if profile == nil {
			continue
		}
		pixels := 0
		if profile.VideoEncoderConfiguration != nil && profile.VideoEncoderConfiguration.Resolution != nil {
			pixels = profile.VideoEncoderConfiguration.Resolution.Width * profile.VideoEncoderConfiguration.Resolution.Height
		}
		if pixels > bestPixels {
			best = profile
			bestPixels = pixels
		}
	}
	return best
}
