package server

import (
	"time"

	onvifsvc "onvif-visor/internal/onvif"
)

const personRecordingPollInterval = time.Second

func (a *App) startPersonRecordingManager() {
	if a.analytics == nil {
		return
	}
	go func() {
		ticker := time.NewTicker(personRecordingPollInterval)
		defer ticker.Stop()
		for {
			select {
			case <-a.ctx.Done():
				return
			case <-ticker.C:
				a.applyPersonRecordings()
			}
		}
	}()
}

func (a *App) applyPersonRecordings() {
	if a.analytics == nil {
		return
	}
	for _, view := range a.store.List() {
		if !view.PersonRecording {
			continue
		}
		if _, ok := a.analytics.RecentPersonFrame(view.ID, 2500*time.Millisecond); !ok {
			continue
		}
		cam, err := a.store.Get(view.ID)
		if err != nil {
			continue
		}
		streamURI, err := onvifsvc.CredentialedRTSP(cam.RTSPURI, cam.Username, cam.Password)
		if err != nil {
			continue
		}
		if _, err := a.media.Trigger(cam.ID, streamURI, triggerDuration(cam)); err != nil {
			logCameraError(cam.ID, "person-record", err)
		}
	}
}
