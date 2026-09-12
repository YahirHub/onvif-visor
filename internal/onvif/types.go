package onvifsvc

import "time"

type DiscoveredDevice struct {
	Name        string   `json:"name"`
	Location    string   `json:"location,omitempty"`
	Endpoint    string   `json:"endpoint"`
	EndpointRef string   `json:"endpoint_ref,omitempty"`
	Scopes      []string `json:"scopes,omitempty"`
}

type ProbeResult struct {
	Endpoint        string
	Manufacturer    string
	Model           string
	FirmwareVersion string
	ProfileToken    string
	ProfileName     string
	RTSPURI         string
	EventsSupported bool
}

type Credentials struct {
	Endpoint string
	Username string
	Password string
}

type Event struct {
	Topic     string            `json:"topic"`
	Time      time.Time         `json:"time"`
	Source    map[string]string `json:"source,omitempty"`
	Data      map[string]string `json:"data,omitempty"`
	Recording bool              `json:"recording_trigger"`
}
