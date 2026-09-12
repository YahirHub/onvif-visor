package onvifsvc

import (
	"context"
	"fmt"
	"strings"
	"time"

	onvif "github.com/0x524a/onvif-go"
)

func WatchEvents(ctx context.Context, creds Credentials, callback func(Event)) error {
	endpoint, err := normalizeHTTPURL(creds.Endpoint)
	if err != nil {
		return err
	}
	client, err := newClient(endpoint, creds.Username, creds.Password)
	if err != nil {
		return err
	}

	initCtx, cancel := context.WithTimeout(ctx, operationTimeout)
	err = client.Initialize(initCtx)
	cancel()
	if err != nil {
		return fmt.Errorf("initialize event service: %w", err)
	}

	for ctx.Err() == nil {
		duration := 5 * time.Minute
		subCtx, subCancel := context.WithTimeout(ctx, operationTimeout)
		sub, subErr := client.CreatePullPointSubscription(subCtx, "", &duration, "")
		subCancel()
		if subErr != nil {
			if !sleepContext(ctx, 3*time.Second) {
				break
			}
			continue
		}

		cycleDeadline := time.Now().Add(4 * time.Minute)
		for ctx.Err() == nil && time.Now().Before(cycleDeadline) {
			messages, pullErr := client.PullMessages(ctx, sub.SubscriptionReference, 5*time.Second, 64)
			if pullErr != nil {
				break
			}
			for _, msg := range messages {
				event := Event{
					Topic:  msg.Topic,
					Time:   msg.Message.UtcTime,
					Source: itemsToMap(msg.Message.Source),
					Data:   itemsToMap(msg.Message.Data),
				}
				if event.Time.IsZero() {
					event.Time = time.Now().UTC()
				}
				event.Recording = IsRecordingEvent(event)
				callback(event)
			}
		}

		unsubCtx, unsubCancel := context.WithTimeout(context.Background(), 5*time.Second)
		_ = client.Unsubscribe(unsubCtx, sub.SubscriptionReference)
		unsubCancel()
		if ctx.Err() == nil {
			_ = sleepContext(ctx, time.Second)
		}
	}
	return ctx.Err()
}

func IsRecordingEvent(event Event) bool {
	topic := strings.ToLower(event.Topic)
	relevantTopic := containsEventTerm(topic)
	if relevantTopic {
		if len(event.Data) == 0 {
			return true
		}
		sawBooleanState := false
		for key, value := range event.Data {
			if !containsEventTerm(strings.ToLower(key)) && !isStateKey(key) {
				continue
			}
			switch normalizeState(value) {
			case "true":
				return true
			case "false":
				sawBooleanState = true
			}
		}
		return !sawBooleanState
	}

	for key, value := range event.Data {
		if !containsEventTerm(strings.ToLower(key)) {
			continue
		}
		return normalizeState(value) != "false"
	}
	return false
}

func containsEventTerm(value string) bool {
	for _, term := range []string{"motion", "cellmotion", "tamper", "digitalinput", "alarm"} {
		if strings.Contains(value, term) {
			return true
		}
	}
	return false
}

func isStateKey(key string) bool {
	key = strings.ToLower(strings.TrimSpace(key))
	return key == "state" || key == "active" || key == "status" || key == "logicalstate"
}

func normalizeState(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "true", "1", "on", "active", "alarm", "motion", "start", "started":
		return "true"
	case "false", "0", "off", "inactive", "idle", "stop", "stopped":
		return "false"
	default:
		return "unknown"
	}
}

func itemsToMap(items []onvif.SimpleItem) map[string]string {
	if len(items) == 0 {
		return nil
	}
	out := make(map[string]string, len(items))
	for _, item := range items {
		out[item.Name] = item.Value
	}
	return out
}

func sleepContext(ctx context.Context, duration time.Duration) bool {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
