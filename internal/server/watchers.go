package server

import (
	"context"
	"strconv"
	"time"

	onvifsvc "onvif-visor/internal/onvif"
	"onvif-visor/internal/store"
)

func (a *App) startExistingWatchers() {
	for _, view := range a.store.List() {
		cam, err := a.store.Get(view.ID)
		if err == nil {
			a.syncWatcher(cam)
		}
	}
}

func (a *App) syncWatcher(cam store.Camera) {
	a.stopWatcher(cam.ID)
	if cam.RTSPURI == "" {
		return
	}

	ctx, cancel := context.WithCancel(a.ctx)
	a.watchMu.Lock()
	a.watchers[cam.ID] = cancel
	a.watchMu.Unlock()

	go func(cameraID string) {
		for ctx.Err() == nil {
			current, err := a.store.Get(cameraID)
			if err != nil {
				return
			}

			if current.EventsSupported {
				err = onvifsvc.WatchEvents(ctx, onvifsvc.Credentials{
					Endpoint: current.Endpoint,
					Username: current.Username,
					Password: current.Password,
				}, func(event onvifsvc.Event) {
					a.onCameraEvent(cameraID, event)
				})
				if ctx.Err() != nil {
					return
				}
				if err != nil {
					logCameraError(cameraID, "events-onvif-fallback", err)
				}
			}

			streamURI, uriErr := onvifsvc.CredentialedRTSP(current.RTSPURI, current.Username, current.Password)
			if uriErr != nil {
				logCameraError(cameraID, "motion-uri", uriErr)
			} else {
				err = a.media.WatchMotion(ctx, streamURI, func(active bool, score float64) {
					event := onvifsvc.Event{
						Topic: "tns1:RuleEngine/SoftwareMotion/Motion",
						Time:  time.Now().UTC(),
						Source: map[string]string{
							"Detector": "software-fallback",
						},
						Data: map[string]string{
							"State": motionState(active),
							"Score": motionScore(score),
						},
						Recording: active,
					}
					a.onCameraEvent(cameraID, event)
				})
				if ctx.Err() != nil {
					return
				}
				if err != nil {
					logCameraError(cameraID, "events-motion-fallback", err)
				}
			}

			select {
			case <-ctx.Done():
				return
			case <-time.After(5 * time.Second):
			}
		}
	}(cam.ID)
}

func motionState(active bool) string {
	if active {
		return "true"
	}
	return "false"
}

func motionScore(score float64) string {
	return strconv.FormatFloat(score, 'f', 4, 64)
}

func (a *App) stopWatcher(cameraID string) {
	a.watchMu.Lock()
	cancel := a.watchers[cameraID]
	delete(a.watchers, cameraID)
	a.watchMu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (a *App) onCameraEvent(cameraID string, event onvifsvc.Event) {
	cam, err := a.store.Get(cameraID)
	if err != nil {
		return
	}

	status := a.media.Status(cam.ID)
	triggered := false
	if cam.EventRecording && event.Recording {
		streamURI, uriErr := onvifsvc.CredentialedRTSP(cam.RTSPURI, cam.Username, cam.Password)
		if uriErr != nil {
			logCameraError(cameraID, "record-uri", uriErr)
		} else if next, triggerErr := a.media.Trigger(cam.ID, streamURI, triggerDuration(cam)); triggerErr != nil {
			logCameraError(cameraID, "record", triggerErr)
		} else {
			status = next
			triggered = true
		}
	}
	a.addEvent(cam, event, status, triggered)
}
