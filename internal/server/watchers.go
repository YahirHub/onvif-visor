package server

import (
	"context"
	"time"

	onvifsvc "onvif-visor/internal/onvif"
	"onvif-visor/internal/store"
)

func (a *App) startExistingWatchers() {
	for _, view := range a.store.List() {
		if !view.EventRecording {
			continue
		}
		cam, err := a.store.Get(view.ID)
		if err == nil {
			a.syncWatcher(cam)
		}
	}
}

func (a *App) syncWatcher(cam store.Camera) {
	a.stopWatcher(cam.ID)
	if !cam.EventRecording || !cam.EventsSupported {
		return
	}

	ctx, cancel := context.WithCancel(a.ctx)
	a.watchMu.Lock()
	a.watchers[cam.ID] = cancel
	a.watchMu.Unlock()

	go func(cameraID string) {
		for ctx.Err() == nil {
			current, err := a.store.Get(cameraID)
			if err != nil || !current.EventRecording {
				return
			}
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
				logCameraError(cameraID, "events", err)
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(5 * time.Second):
			}
		}
	}(cam.ID)
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
	a.addEvent(cam, event)
	if !cam.EventRecording || !event.Recording {
		return
	}
	streamURI, err := onvifsvc.CredentialedRTSP(cam.RTSPURI, cam.Username, cam.Password)
	if err != nil {
		logCameraError(cameraID, "record-uri", err)
		return
	}
	if _, err := a.media.Trigger(cam.ID, streamURI, triggerDuration(cam)); err != nil {
		logCameraError(cameraID, "record", err)
	}
}
